package antirepeat

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var ErrNoSnapshot = errors.New("anti-repeat snapshot does not exist")

const snapshotVersion = 1

type savedBucket struct {
	Start   int64
	Bits    []byte
	Inserts uint64
}

type savedStream struct {
	Key         StreamKey
	UserKey     string
	WindowHours int
	Buckets     []savedBucket
}

type savedState struct {
	Version       int
	SavedAt       time.Time
	BitsPerBucket uint64
	HashCount     uint64
	ExpectedKeys  int
	BucketCount   int
	Streams       []savedStream
}

func (m *Manager) Save() (resultErr error) {
	m.mu.RLock()
	states := make(map[StreamKey]*streamState, len(m.streams))
	for key, state := range m.streams {
		states[key] = state
	}
	m.mu.RUnlock()
	saved := savedState{
		Version:       snapshotVersion,
		SavedAt:       time.Now().UTC(),
		BitsPerBucket: m.bitsPerBucket,
		HashCount:     m.hashCount,
		ExpectedKeys:  m.config.ExpectedKeysPerBucket,
		BucketCount:   m.config.BucketCount,
		Streams:       make([]savedStream, 0, len(states)),
	}
	for key, state := range states {
		state.mu.Lock()
		m.prune(state, saved.SavedAt)
		item := savedStream{
			Key:         key,
			UserKey:     state.policy.UserKey,
			WindowHours: state.policy.WindowHours,
			Buckets:     make([]savedBucket, 0, len(state.buckets)),
		}
		for _, current := range state.buckets {
			item.Buckets = append(item.Buckets, savedBucket{
				Start:   current.Start,
				Bits:    append([]byte(nil), current.Bits...),
				Inserts: current.Inserts,
			})
		}
		state.mu.Unlock()
		saved.Streams = append(saved.Streams, item)
	}
	var payload bytes.Buffer
	if err := gob.NewEncoder(&payload).Encode(saved); err != nil {
		return err
	}
	checksum := sha256.Sum256(payload.Bytes())
	path := m.config.SnapshotPath
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".bloom-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() {
		if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	if err := file.Chmod(0600); err != nil {
		return errors.Join(err, file.Close())
	}
	if _, err := file.Write(checksum[:]); err != nil {
		return errors.Join(err, file.Close())
	}
	if _, err := file.Write(payload.Bytes()); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	m.lastSaved.Store(saved.SavedAt.Unix())
	return nil
}

func (m *Manager) Load() (resultErr error) {
	file, err := os.Open(m.config.SnapshotPath)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNoSnapshot
	}
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	maxFileBytes := m.config.MaxBytes + 16*1024*1024
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxFileBytes || len(data) < sha256.Size {
		return errors.New("anti-repeat snapshot is too large or truncated")
	}
	wantChecksum := sha256.Sum256(data[sha256.Size:])
	if !bytes.Equal(data[:sha256.Size], wantChecksum[:]) {
		return errors.New("anti-repeat snapshot checksum mismatch")
	}
	var saved savedState
	if err := gob.NewDecoder(bytes.NewReader(data[sha256.Size:])).Decode(&saved); err != nil {
		return err
	}
	if saved.Version != snapshotVersion || saved.BitsPerBucket != m.bitsPerBucket ||
		saved.HashCount != m.hashCount || saved.ExpectedKeys != m.config.ExpectedKeysPerBucket ||
		saved.BucketCount != m.config.BucketCount {
		return errors.New("anti-repeat snapshot configuration mismatch")
	}
	now := time.Now().UTC()
	for _, item := range saved.Streams {
		m.mu.RLock()
		state := m.streams[item.Key]
		if state == nil {
			m.mu.RUnlock()
			continue
		}
		state.mu.Lock()
		m.mu.RUnlock()
		if state.policy != (policy{item.UserKey, item.WindowHours}) {
			state.mu.Unlock()
			continue
		}
		for _, persisted := range item.Buckets {
			if len(persisted.Bits) != int(m.bitsPerBucket/8) ||
				persisted.Start != m.bucketStart(state, time.Unix(persisted.Start, 0)) {
				state.mu.Unlock()
				return fmt.Errorf("invalid anti-repeat bucket for stream %s", item.Key.StreamID)
			}
			if _, exists := state.buckets[persisted.Start]; exists {
				continue
			}
			if persisted.Start+m.bucketDuration(state) <= now.Unix()-int64(state.policy.WindowHours)*3600 {
				continue
			}
			bytes := int64(len(persisted.Bits))
			if m.usedBytes.Add(bytes) > m.config.MaxBytes {
				m.usedBytes.Add(-bytes)
				state.mu.Unlock()
				return ErrHistoryUnavailable
			}
			state.buckets[persisted.Start] = &bucket{
				Start:   persisted.Start,
				Bits:    persisted.Bits,
				Inserts: persisted.Inserts,
			}
		}
		m.prune(state, now)
		state.mu.Unlock()
	}
	m.lastSaved.Store(saved.SavedAt.Unix())
	return nil
}
