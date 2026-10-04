package eventqueue

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

const maxWALRecord = 1 << 20

var (
	ErrWALFull  = errors.New("click WAL capacity reached")
	ErrWALWrite = errors.New("click WAL write failed")
)

type ClickWALConfig struct {
	Dir          string
	MaxBytes     int64
	SegmentBytes int64
	BatchSize    int
	RetryDelay   time.Duration
}

type walPosition struct {
	Segment uint64 `json:"segment"`
	Offset  int64  `json:"offset"`
}

type ClickWAL struct {
	mu              sync.Mutex
	cfg             ClickWALConfig
	log             *slog.Logger
	write           WriteBatchFunc[models.ClickEvent]
	files           map[uint64]int64
	active          *os.File
	writePos        walPosition
	readPos         walPosition
	diskBytes       int64
	pending         int64
	accepted        uint64
	delivered       uint64
	failed          uint64
	overflow        uint64
	retries         uint64
	recovered       int64
	recoveryEnd     walPosition
	recoveryPending bool
	recoveryFrom    time.Time
	recoveryTo      time.Time
	closed          bool
	wake            chan struct{}
	stop            chan struct{}
	done            chan struct{}
}

func OpenClickWAL(
	cfg ClickWALConfig,
	log *slog.Logger,
	write WriteBatchFunc[models.ClickEvent],
) (*ClickWAL, error) {
	if cfg.Dir == "" || cfg.MaxBytes <= 0 || cfg.SegmentBytes <= 0 || cfg.SegmentBytes > cfg.MaxBytes || cfg.BatchSize <= 0 || cfg.RetryDelay <= 0 || write == nil {
		return nil, errors.New("invalid click WAL config")
	}
	if err := os.MkdirAll(cfg.Dir, 0700); err != nil {
		return nil, fmt.Errorf("create click WAL directory: %w", err)
	}
	if log == nil {
		log = slog.Default()
	}
	w := &ClickWAL{
		cfg:   cfg,
		log:   log,
		write: write,
		files: make(map[uint64]int64),
		wake:  make(chan struct{}, 1),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	if err := w.recover(); err != nil {
		return nil, err
	}
	go w.run()
	return w, nil
}

func (w *ClickWAL) Log(
	ctx context.Context,
	event models.ClickEvent,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		w.countInvalid(err)
		return fmt.Errorf("%w: encode click: %v", ErrWALWrite, err)
	}
	if len(data) > maxWALRecord {
		err := fmt.Errorf("click WAL record exceeds %d bytes", maxWALRecord)
		w.countInvalid(err)
		return fmt.Errorf("%w: %v", ErrWALWrite, err)
	}
	record := make([]byte, 8+len(data))
	binary.BigEndian.PutUint32(record[:4], uint32(len(data)))
	binary.BigEndian.PutUint32(record[4:8], crc32.ChecksumIEEE(data))
	copy(record[8:], data)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return ErrClosed
	}
	if w.writePos.Offset+int64(len(record)) > w.cfg.SegmentBytes && w.writePos.Offset > 0 {
		if err := w.rotate(); err != nil {
			w.failed++
			w.warnDrop("click WAL rotation failed", err, w.failed)
			return fmt.Errorf("%w: %v", ErrWALWrite, err)
		}
	}
	if w.diskBytes+int64(len(record)) > w.cfg.MaxBytes {
		if w.readPos == w.writePos && w.writePos.Offset > 0 {
			if err := w.rotate(); err != nil {
				w.failed++
				w.warnDrop("click WAL rotation failed", err, w.failed)
				return fmt.Errorf("%w: %v", ErrWALWrite, err)
			}
		}
		w.reclaim()
		if w.diskBytes+int64(len(record)) > w.cfg.MaxBytes {
			w.overflow++
			w.warnDrop("click WAL full; click event lost", ErrWALFull, w.overflow)
			return ErrWALFull
		}
	}
	oldOffset := w.writePos.Offset
	if _, err := w.active.Write(record); err != nil {
		w.rollback(oldOffset)
		w.failed++
		w.warnDrop("click WAL append failed", err, w.failed)
		return fmt.Errorf("%w: %v", ErrWALWrite, err)
	}
	// A redirect follows only after the local record is durable.
	if err := w.active.Sync(); err != nil {
		w.rollback(oldOffset)
		w.failed++
		w.warnDrop("click WAL sync failed", err, w.failed)
		return fmt.Errorf("%w: %v", ErrWALWrite, err)
	}
	w.writePos.Offset += int64(len(record))
	w.files[w.writePos.Segment] = w.writePos.Offset
	w.diskBytes += int64(len(record))
	w.pending++
	w.accepted++
	select {
	case w.wake <- struct{}{}:
	default:
	}
	return nil
}

func (w *ClickWAL) Stats() Stats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return Stats{
		Pending:         w.pending,
		AcceptedTotal:   w.accepted,
		DeliveredTotal:  w.delivered,
		FailedTotal:     w.failed,
		OverflowTotal:   w.overflow,
		Closed:          w.closed,
		DiskBytes:       w.diskBytes,
		MaxDiskBytes:    w.cfg.MaxBytes,
		RetryTotal:      w.retries,
		RecoveredTotal:  w.recovered,
		RecoveryPending: w.recoveryPending,
	}
}

// RecoveryWindow identifies event dates that may be replayed after this start.
func (w *ClickWAL) RecoveryWindow() (time.Time, time.Time, int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.recoveryFrom, w.recoveryTo, w.recovered
}

func (w *ClickWAL) warnDrop(message string, err error, count uint64) {
	if w.log != nil && (count == 1 || count%1000 == 0) {
		w.log.Error(message, "error", err, "dropped_total", count)
	}
}

func (w *ClickWAL) countInvalid(err error) {
	w.mu.Lock()
	w.failed++
	w.warnDrop("click WAL event rejected", err, w.failed)
	w.mu.Unlock()
}

func (w *ClickWAL) Close(ctx context.Context) error {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.stop)
	}
	w.mu.Unlock()
	select {
	case <-w.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active.Close()
}

func (w *ClickWAL) Done() <-chan struct{} { return w.done }

func (w *ClickWAL) run() {
	defer close(w.done)
	for {
		batch, next, err := w.nextBatch()
		if err != nil {
			w.log.Error("click WAL read failed", "error", err)
			if !w.waitRetry() {
				return
			}
			continue
		}
		if len(batch) == 0 {
			select {
			case <-w.stop:
				return
			case <-w.wake:
				continue
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		stopped := make(chan struct{})
		go func() {
			select {
			case <-w.stop:
				cancel()
			case <-stopped:
			}
		}()
		err = w.write(ctx, batch)
		close(stopped)
		cancel()
		if err != nil {
			w.mu.Lock()
			w.retries++
			retries := w.retries
			w.mu.Unlock()
			if retries == 1 || retries%100 == 0 {
				w.log.Warn("click WAL delivery will retry", "error", err, "retry_total", retries)
			}
			if !w.waitRetry() {
				return
			}
			continue
		}
		w.mu.Lock()
		if err := w.checkpoint(next); err != nil {
			w.failed++
			w.mu.Unlock()
			w.log.Error("click WAL checkpoint failed; batch will replay", "error", err)
			if !w.waitRetry() {
				return
			}
			continue
		}
		w.readPos = next
		if w.recoveryPending && positionAtOrAfter(next, w.recoveryEnd) {
			w.recoveryPending = false
		}
		w.delivered += uint64(len(batch))
		w.pending -= int64(len(batch))
		w.reclaim()
		w.mu.Unlock()
	}
}

func (w *ClickWAL) waitRetry() bool {
	select {
	case <-w.stop:
		return false
	case <-time.After(w.cfg.RetryDelay):
		return true
	}
}

func (w *ClickWAL) nextBatch() ([]models.ClickEvent, walPosition, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pos := w.readPos
	batch := make([]models.ClickEvent, 0, w.cfg.BatchSize)
	for len(batch) < w.cfg.BatchSize {
		if pos.Segment == w.writePos.Segment && pos.Offset == w.writePos.Offset {
			break
		}
		size, ok := w.files[pos.Segment]
		if !ok {
			return nil, pos, errors.New("missing click WAL segment")
		}
		if pos.Offset == size {
			pos = walPosition{Segment: pos.Segment + 1}
			continue
		}
		file, err := os.Open(w.segmentPath(pos.Segment))
		if err != nil {
			return nil, pos, err
		}
		for len(batch) < w.cfg.BatchSize && pos.Offset < size {
			var header [8]byte
			if _, err := file.ReadAt(header[:], pos.Offset); err != nil {
				return nil, pos, errors.Join(err, file.Close())
			}
			length := binary.BigEndian.Uint32(header[:4])
			if length == 0 || length > maxWALRecord || pos.Offset+8+int64(length) > size {
				return nil, pos, errors.Join(errors.New("invalid click WAL record length"), file.Close())
			}
			data := make([]byte, length)
			if _, err := file.ReadAt(data, pos.Offset+8); err != nil {
				return nil, pos, errors.Join(err, file.Close())
			}
			if crc32.ChecksumIEEE(data) != binary.BigEndian.Uint32(header[4:8]) {
				return nil, pos, errors.Join(errors.New("click WAL checksum mismatch"), file.Close())
			}
			var event models.ClickEvent
			if err := json.Unmarshal(data, &event); err != nil {
				return nil, pos, errors.Join(err, file.Close())
			}
			batch = append(batch, event)
			pos.Offset += 8 + int64(length)
		}
		if err := file.Close(); err != nil {
			return nil, pos, err
		}
	}
	return batch, pos, nil
}

func (w *ClickWAL) recover() error {
	entries, err := os.ReadDir(w.cfg.Dir)
	if err != nil {
		return err
	}
	var segments []uint64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wal") {
			continue
		}
		seq, err := strconv.ParseUint(strings.TrimSuffix(entry.Name(), ".wal"), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid click WAL segment %q: %w", entry.Name(), err)
		}
		segments = append(segments, seq)
	}
	sort.Slice(segments, func(i, j int) bool { return segments[i] < segments[j] })
	if len(segments) == 0 {
		segments = append(segments, 1)
	}
	for i, seq := range segments {
		if i > 0 && seq != segments[i-1]+1 {
			return errors.New("click WAL segment gap")
		}
		path := w.segmentPath(seq)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return err
		}
		if err := syncDir(w.cfg.Dir); err != nil {
			return errors.Join(err, file.Close())
		}
		size, count, err := scanWAL(file, i == len(segments)-1)
		if err != nil {
			return errors.Join(err, file.Close())
		}
		w.files[seq] = size
		w.diskBytes += size
		w.pending += count
		if i == len(segments)-1 {
			w.active = file
			w.writePos = walPosition{Segment: seq, Offset: size}
			if _, err := file.Seek(size, io.SeekStart); err != nil {
				return errors.Join(err, file.Close())
			}
		} else if err := file.Close(); err != nil {
			return err
		}
	}
	w.readPos = walPosition{Segment: segments[0]}
	data, err := os.ReadFile(filepath.Join(w.cfg.Dir, "checkpoint.json"))
	if err == nil {
		var saved walPosition
		if err := json.Unmarshal(data, &saved); err != nil {
			return err
		}
		if saved.Segment > w.writePos.Segment || saved.Segment < segments[0]-1 {
			return errors.New("click WAL checkpoint outside segment range")
		}
		if saved.Segment < segments[0] {
			w.readPos = walPosition{Segment: segments[0]}
		} else {
			if saved.Offset > w.files[saved.Segment] {
				return errors.New("click WAL checkpoint exceeds segment")
			}
			w.readPos = saved
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Count records after the durable checkpoint, including those in later segments.
	w.pending = 0
	for _, seq := range segments {
		if seq < w.readPos.Segment {
			continue
		}
		file, err := os.Open(w.segmentPath(seq))
		if err != nil {
			return err
		}
		start := int64(0)
		if seq == w.readPos.Segment {
			start = w.readPos.Offset
		}
		_, count, err := scanWALFrom(
			file,
			start,
			false,
			func(data []byte) error {
				var event struct {
					CreatedAt time.Time `json:"created_at"`
				}
				if err := json.Unmarshal(data, &event); err != nil {
					return err
				}
				if event.CreatedAt.IsZero() {
					return nil
				}
				if w.recoveryFrom.IsZero() || event.CreatedAt.Before(w.recoveryFrom) {
					w.recoveryFrom = event.CreatedAt
				}
				if event.CreatedAt.After(w.recoveryTo) {
					w.recoveryTo = event.CreatedAt
				}
				return nil
			},
		)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return errors.Join(err, closeErr)
		}
		w.pending += count
	}
	w.recovered = w.pending
	w.recoveryEnd = w.writePos
	w.recoveryPending = w.recovered > 0
	w.reclaim()
	return nil
}

func positionAtOrAfter(current walPosition, target walPosition) bool {
	return current.Segment > target.Segment ||
		(current.Segment == target.Segment && current.Offset >= target.Offset)
}

func scanWAL(file *os.File, repairTail bool) (int64, int64, error) {
	return scanWALFrom(file, 0, repairTail, nil)
}

func scanWALFrom(
	file *os.File,
	start int64,
	repairTail bool,
	onRecord func([]byte) error,
) (int64, int64, error) {
	info, err := file.Stat()
	if err != nil {
		return 0, 0, err
	}
	pos := start
	var count int64
	for pos < info.Size() {
		var header [8]byte
		_, err := file.ReadAt(header[:], pos)
		if err != nil {
			if repairTail && errors.Is(err, io.EOF) {
				return truncateWAL(file, pos, count)
			}
			return 0, 0, err
		}
		length := binary.BigEndian.Uint32(header[:4])
		if length == 0 || length > maxWALRecord {
			return 0, 0, errors.New("invalid click WAL record length")
		}
		if pos+8+int64(length) > info.Size() {
			if repairTail {
				return truncateWAL(file, pos, count)
			}
			return 0, 0, io.ErrUnexpectedEOF
		}
		data := make([]byte, length)
		if _, err := file.ReadAt(data, pos+8); err != nil {
			return 0, 0, err
		}
		if crc32.ChecksumIEEE(data) != binary.BigEndian.Uint32(header[4:8]) {
			return 0, 0, errors.New("click WAL checksum mismatch")
		}
		if onRecord != nil {
			if err := onRecord(data); err != nil {
				return 0, 0, err
			}
		}
		pos += 8 + int64(length)
		count++
	}
	return pos, count, nil
}

func truncateWAL(file *os.File, pos int64, count int64) (int64, int64, error) {
	if err := file.Truncate(pos); err != nil {
		return 0, 0, err
	}
	if err := file.Sync(); err != nil {
		return 0, 0, err
	}
	return pos, count, nil
}

func (w *ClickWAL) segmentPath(seq uint64) string {
	return filepath.Join(w.cfg.Dir, fmt.Sprintf("%020d.wal", seq))
}

func (w *ClickWAL) rotate() error {
	seq := w.writePos.Segment + 1
	file, err := os.OpenFile(w.segmentPath(seq), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	if err := syncDir(w.cfg.Dir); err != nil {
		return errors.Join(
			err,
			file.Close(),
			os.Remove(w.segmentPath(seq)),
		)
	}
	if err := w.active.Close(); err != nil {
		if w.log != nil {
			w.log.Warn("previous click WAL segment close failed", "error", err)
		}
	}
	if w.readPos == w.writePos {
		w.readPos = walPosition{Segment: seq}
	}
	w.active = file
	w.writePos = walPosition{Segment: seq}
	w.files[seq] = 0
	w.reclaim()
	return nil
}

func (w *ClickWAL) rollback(offset int64) {
	if err := w.active.Truncate(offset); err != nil && w.log != nil {
		w.log.Error("click WAL rollback failed", "error", err)
	}
	if _, err := w.active.Seek(offset, io.SeekStart); err != nil && w.log != nil {
		w.log.Error("click WAL seek after rollback failed", "error", err)
	}
}

func (w *ClickWAL) checkpoint(pos walPosition) error {
	data, err := json.Marshal(pos)
	if err != nil {
		return err
	}
	path := filepath.Join(w.cfg.Dir, "checkpoint.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	file, err := os.OpenFile(tmp, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDir(w.cfg.Dir)
}

func (w *ClickWAL) reclaim() {
	for seq, size := range w.files {
		if seq >= w.readPos.Segment {
			continue
		}
		if err := os.Remove(w.segmentPath(seq)); err != nil {
			if w.log != nil {
				w.log.Warn("click WAL segment cleanup failed", "error", err)
			}
			continue
		}
		delete(w.files, seq)
		w.diskBytes -= size
	}
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := dir.Sync(); err != nil {
		return errors.Join(err, dir.Close())
	}
	return dir.Close()
}
