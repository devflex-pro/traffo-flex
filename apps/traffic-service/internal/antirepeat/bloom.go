package antirepeat

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var ErrHistoryUnavailable = errors.New("anti-repeat history is unavailable")
var ErrHistoryExhausted = errors.New("anti-repeat destinations are exhausted")

type Config struct {
	ExpectedKeysPerBucket int
	FalsePositiveRate     float64
	MaxBytes              int64
	BucketCount           int
	SnapshotPath          string
	SnapshotInterval      time.Duration
}

func (c Config) Validate() error {
	if c.ExpectedKeysPerBucket <= 0 || c.BucketCount < 2 || c.BucketCount > 64 ||
		c.FalsePositiveRate <= 0 || c.FalsePositiveRate >= 0.1 || math.IsNaN(c.FalsePositiveRate) ||
		c.MaxBytes <= 0 ||
		c.SnapshotPath == "" || c.SnapshotInterval <= 0 {
		return errors.New("invalid anti-repeat Bloom configuration")
	}
	return nil
}

type StreamKey struct {
	CampaignID string
	StreamID   string
}

type policy struct {
	UserKey     string
	WindowHours int
}

type bucket struct {
	Start   int64
	Bits    []byte
	Inserts uint64
}

type streamState struct {
	mu      sync.Mutex
	policy  policy
	ready   bool
	buckets map[int64]*bucket
}

type Stats struct {
	Streams           int       `json:"streams"`
	ReadyStreams      int       `json:"ready_streams"`
	PendingStreams    int       `json:"pending_streams"`
	UsedBytes         int64     `json:"used_bytes"`
	MaxBytes          int64     `json:"max_bytes"`
	FalsePositiveRate float64   `json:"false_positive_rate"`
	MissingTotal      uint64    `json:"missing_total"`
	ExhaustedTotal    uint64    `json:"exhausted_total"`
	SaturatedTotal    uint64    `json:"saturated_total"`
	LastSavedAt       time.Time `json:"last_saved_at,omitempty"`
	LastSeedError     string    `json:"last_seed_error,omitempty"`
}

type Manager struct {
	config        Config
	mu            sync.RWMutex
	streams       map[StreamKey]*streamState
	usedBytes     atomic.Int64
	missing       atomic.Uint64
	exhausted     atomic.Uint64
	saturated     atomic.Uint64
	lastSaved     atomic.Int64
	lastSeedMu    sync.RWMutex
	lastSeedError string
	bitsPerBucket uint64
	hashCount     uint64
}

func NewManager(config Config) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	perBucketRate := config.FalsePositiveRate / float64(config.BucketCount+1)
	bits := uint64(math.Ceil(-float64(config.ExpectedKeysPerBucket) *
		math.Log(perBucketRate) / (math.Ln2 * math.Ln2)))
	if bits < 64 {
		bits = 64
	}
	bits = (bits + 7) / 8 * 8
	if bits/8 > uint64(config.MaxBytes) {
		return nil, errors.New("anti-repeat Bloom bucket exceeds memory budget")
	}
	hashes := uint64(math.Round(float64(bits) / float64(config.ExpectedKeysPerBucket) * math.Ln2))
	if hashes < 1 {
		hashes = 1
	}
	return &Manager{
		config:        config,
		streams:       make(map[StreamKey]*streamState),
		bitsPerBucket: bits,
		hashCount:     hashes,
	}, nil
}

func (m *Manager) Configure(campaigns []cache.CampaignConfig) {
	wanted := make(map[StreamKey]policy)
	for _, campaign := range campaigns {
		if campaign.Campaign.Status != models.StatusActive {
			continue
		}
		for _, stream := range campaign.Streams {
			unique := stream.Distribution.UniquePolicy
			if stream.Status != models.StatusActive || !unique.Enabled {
				continue
			}
			if err := models.ValidateUniqueDestinationPolicy(unique); err != nil {
				continue
			}
			wanted[StreamKey{campaign.Campaign.ID, stream.ID}] = policy{
				UserKey:     unique.UserKey,
				WindowHours: unique.HistoryWindowHours,
			}
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, state := range m.streams {
		if desired, ok := wanted[key]; ok && desired == state.policy {
			continue
		}
		state.mu.Lock()
		for _, item := range state.buckets {
			m.usedBytes.Add(-int64(len(item.Bits)))
		}
		state.mu.Unlock()
		delete(m.streams, key)
	}
	for key, desired := range wanted {
		if _, ok := m.streams[key]; !ok {
			m.streams[key] = &streamState{
				policy:  desired,
				buckets: make(map[int64]*bucket),
			}
		}
	}
}

func (m *Manager) SelectAndMark(
	key StreamKey,
	unique models.UniqueDestinationPolicy,
	userValue string,
	destinations []models.Destination,
	now time.Time,
	selectDestination func([]models.Destination) (models.Destination, error),
) (models.Destination, error) {
	if userValue == "" {
		return selectDestination(destinations)
	}
	m.mu.RLock()
	state := m.streams[key]
	if state == nil {
		m.mu.RUnlock()
		m.missing.Add(1)
		return models.Destination{}, ErrHistoryUnavailable
	}
	state.mu.Lock()
	m.mu.RUnlock()
	defer state.mu.Unlock()
	if !state.ready || state.policy != (policy{unique.UserKey, unique.HistoryWindowHours}) {
		m.missing.Add(1)
		return models.Destination{}, ErrHistoryUnavailable
	}
	m.prune(state, now)
	current, err := m.bucketFor(state, now)
	if err != nil {
		m.saturated.Add(1)
		return models.Destination{}, ErrHistoryUnavailable
	}
	if current.Inserts >= uint64(m.config.ExpectedKeysPerBucket) {
		m.saturated.Add(1)
		return models.Destination{}, ErrHistoryUnavailable
	}
	unseen := make([]models.Destination, 0, len(destinations))
	for _, destination := range destinations {
		fingerprint := fingerprint(
			key,
			destination.ID,
			unique.UserKey,
			userValue,
		)
		if !m.seen(state, fingerprint) {
			unseen = append(unseen, destination)
		}
	}
	if len(unseen) == 0 {
		if unique.ExhaustedMode != models.UniqueExhaustedAllowRepeat && unique.ExhaustedMode != "" {
			m.exhausted.Add(1)
			return models.Destination{}, ErrHistoryExhausted
		}
		unseen = destinations
	}
	selected, err := selectDestination(unseen)
	if err != nil {
		return models.Destination{}, err
	}
	fingerprint := fingerprint(key, selected.ID, unique.UserKey, userValue)
	if !m.contains(current, fingerprint) {
		m.add(current, fingerprint)
		current.Inserts++
	}
	return selected, nil
}

func (m *Manager) MarkHistorical(
	key StreamKey,
	userKey string,
	userValue string,
	destinationID string,
	at time.Time,
) error {
	if userValue == "" || destinationID == "" {
		return nil
	}
	m.mu.RLock()
	state := m.streams[key]
	if state == nil {
		m.mu.RUnlock()
		return ErrHistoryUnavailable
	}
	state.mu.Lock()
	m.mu.RUnlock()
	defer state.mu.Unlock()
	if state.policy.UserKey != userKey {
		return ErrHistoryUnavailable
	}
	m.prune(state, time.Now().UTC())
	if at.Before(time.Now().Add(-time.Duration(state.policy.WindowHours) * time.Hour)) {
		return nil
	}
	item, err := m.bucketFor(state, at)
	if err != nil {
		return err
	}
	fingerprint := fingerprint(key, destinationID, userKey, userValue)
	if !m.contains(item, fingerprint) {
		m.add(item, fingerprint)
		item.Inserts++
	}
	if item.Inserts > uint64(m.config.ExpectedKeysPerBucket) {
		return ErrHistoryUnavailable
	}
	return nil
}

func (m *Manager) SetReady(key StreamKey, userKey string, windowHours int) bool {
	m.mu.RLock()
	state := m.streams[key]
	if state == nil {
		m.mu.RUnlock()
		return false
	}
	state.mu.Lock()
	m.mu.RUnlock()
	defer state.mu.Unlock()
	if state.policy != (policy{userKey, windowHours}) {
		return false
	}
	state.ready = true
	return true
}

func (m *Manager) Pending() []PendingStream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]PendingStream, 0)
	for key, state := range m.streams {
		state.mu.Lock()
		if !state.ready {
			items = append(items, PendingStream{key, state.policy.UserKey, state.policy.WindowHours})
		}
		state.mu.Unlock()
	}
	return items
}

type PendingStream struct {
	Key         StreamKey
	UserKey     string
	WindowHours int
}

func (m *Manager) SetSeedError(err error) {
	m.lastSeedMu.Lock()
	if err == nil {
		m.lastSeedError = ""
	} else {
		m.lastSeedError = err.Error()
	}
	m.lastSeedMu.Unlock()
}

func (m *Manager) Stats() Stats {
	m.mu.RLock()
	streamCount := len(m.streams)
	readyCount := 0
	for _, state := range m.streams {
		state.mu.Lock()
		if state.ready {
			readyCount++
		}
		state.mu.Unlock()
	}
	m.mu.RUnlock()
	stats := Stats{
		Streams:           streamCount,
		ReadyStreams:      readyCount,
		PendingStreams:    streamCount - readyCount,
		UsedBytes:         m.usedBytes.Load(),
		MaxBytes:          m.config.MaxBytes,
		FalsePositiveRate: m.config.FalsePositiveRate,
		MissingTotal:      m.missing.Load(),
		ExhaustedTotal:    m.exhausted.Load(),
		SaturatedTotal:    m.saturated.Load(),
	}
	if unix := m.lastSaved.Load(); unix > 0 {
		stats.LastSavedAt = time.Unix(unix, 0).UTC()
	}
	m.lastSeedMu.RLock()
	stats.LastSeedError = m.lastSeedError
	m.lastSeedMu.RUnlock()
	return stats
}

func (m *Manager) bucketFor(state *streamState, at time.Time) (*bucket, error) {
	start := m.bucketStart(state, at)
	if existing := state.buckets[start]; existing != nil {
		return existing, nil
	}
	bytes := int64(m.bitsPerBucket / 8)
	for {
		used := m.usedBytes.Load()
		if used+bytes > m.config.MaxBytes {
			return nil, ErrHistoryUnavailable
		}
		if m.usedBytes.CompareAndSwap(used, used+bytes) {
			break
		}
	}
	item := &bucket{Start: start, Bits: make([]byte, bytes)}
	state.buckets[start] = item
	return item, nil
}

func (m *Manager) bucketStart(state *streamState, at time.Time) int64 {
	duration := m.bucketDuration(state)
	return at.Unix() / duration * duration
}

func (m *Manager) bucketDuration(state *streamState) int64 {
	windowSeconds := int64(state.policy.WindowHours) * 3600
	duration := (windowSeconds + int64(m.config.BucketCount) - 1) / int64(m.config.BucketCount)
	duration = (duration + 3599) / 3600 * 3600
	return duration
}

func (m *Manager) prune(state *streamState, now time.Time) {
	windowSeconds := int64(state.policy.WindowHours) * 3600
	duration := m.bucketDuration(state)
	cutoff := now.Unix() - windowSeconds
	for start, item := range state.buckets {
		if start+duration <= cutoff {
			delete(state.buckets, start)
			m.usedBytes.Add(-int64(len(item.Bits)))
		}
	}
}

func (m *Manager) seen(state *streamState, hash [32]byte) bool {
	for _, item := range state.buckets {
		if m.contains(item, hash) {
			return true
		}
	}
	return false
}

func (m *Manager) contains(item *bucket, hash [32]byte) bool {
	h1 := binary.LittleEndian.Uint64(hash[:8])
	h2 := binary.LittleEndian.Uint64(hash[8:16]) | 1
	for index := uint64(0); index < m.hashCount; index++ {
		bit := (h1 + index*h2) % m.bitsPerBucket
		if item.Bits[bit/8]&(1<<(bit%8)) == 0 {
			return false
		}
	}
	return true
}

func (m *Manager) add(item *bucket, hash [32]byte) {
	h1 := binary.LittleEndian.Uint64(hash[:8])
	h2 := binary.LittleEndian.Uint64(hash[8:16]) | 1
	for index := uint64(0); index < m.hashCount; index++ {
		bit := (h1 + index*h2) % m.bitsPerBucket
		item.Bits[bit/8] |= 1 << (bit % 8)
	}
}

func fingerprint(key StreamKey, destinationID string, userKey string, userValue string) [32]byte {
	data := make([]byte, 0, len(key.CampaignID)+len(key.StreamID)+len(destinationID)+len(userKey)+len(userValue)+5)
	for _, part := range []string{key.CampaignID, key.StreamID, destinationID, userKey, userValue} {
		data = binary.LittleEndian.AppendUint32(data, uint32(len(part)))
		data = append(data, part...)
	}
	return sha256.Sum256(data)
}
