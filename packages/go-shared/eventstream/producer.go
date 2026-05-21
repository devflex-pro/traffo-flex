package eventstream

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
)

var ErrNoBrokers = errors.New("eventstream brokers are required")

type Config struct {
	Brokers      []string
	ClientID     string
	BatchSize    int
	BatchTimeout time.Duration
	WriteTimeout time.Duration
	MaxAttempts  int
	RetryBackoff time.Duration
	Logger       *slog.Logger
}

type Producer struct {
	writer       writer
	log          *slog.Logger
	clientID     string
	writeTimeout time.Duration
	maxAttempts  int
	retryBackoff time.Duration
	stats        producerStats
}

type writer interface {
	WriteMessages(
		ctx context.Context,
		msgs ...kafka.Message,
	) error
	Close() error
}

type Stats struct {
	ClientID            string `json:"client_id"`
	WriteCallsTotal     uint64 `json:"write_calls_total"`
	WriteAttemptsTotal  uint64 `json:"write_attempts_total"`
	WriteSuccessTotal   uint64 `json:"write_success_total"`
	WriteFailureTotal   uint64 `json:"write_failure_total"`
	WriteRetryTotal     uint64 `json:"write_retry_total"`
	MarshalFailureTotal uint64 `json:"marshal_failure_total"`
	ContextCancelTotal  uint64 `json:"context_cancel_total"`
	BytesWrittenTotal   uint64 `json:"bytes_written_total"`
	LastSuccessAt       string `json:"last_success_at,omitempty"`
	LastFailureAt       string `json:"last_failure_at,omitempty"`
	LastError           string `json:"last_error,omitempty"`
}

type producerStats struct {
	writeCalls     atomic.Uint64
	writeAttempts  atomic.Uint64
	writeSuccess   atomic.Uint64
	writeFailure   atomic.Uint64
	writeRetry     atomic.Uint64
	marshalFailure atomic.Uint64
	contextCancel  atomic.Uint64
	bytesWritten   atomic.Uint64
	lastSuccessMS  atomic.Int64
	lastFailureMS  atomic.Int64
	mu             sync.RWMutex
	lastError      string
}

func NewProducer(cfg Config) (
	*Producer,
	error,
) {
	if len(cfg.Brokers) == 0 {
		return nil, ErrNoBrokers
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.BatchTimeout <= 0 {
		cfg.BatchTimeout = time.Second
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 3 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 100 * time.Millisecond
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.Hash{},
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		Transport: &kafka.Transport{
			ClientID: cfg.ClientID,
		},
	}
	return newProducerWithWriter(
		cfg,
		writer,
	), nil
}

func newProducerWithWriter(
	cfg Config,
	writer writer,
) *Producer {
	return &Producer{
		writer:       writer,
		log:          cfg.Logger,
		clientID:     cfg.ClientID,
		writeTimeout: cfg.WriteTimeout,
		maxAttempts:  cfg.MaxAttempts,
		retryBackoff: cfg.RetryBackoff,
	}
}

func (p *Producer) Stats() Stats {
	stats := Stats{
		ClientID:            p.clientID,
		WriteCallsTotal:     p.stats.writeCalls.Load(),
		WriteAttemptsTotal:  p.stats.writeAttempts.Load(),
		WriteSuccessTotal:   p.stats.writeSuccess.Load(),
		WriteFailureTotal:   p.stats.writeFailure.Load(),
		WriteRetryTotal:     p.stats.writeRetry.Load(),
		MarshalFailureTotal: p.stats.marshalFailure.Load(),
		ContextCancelTotal:  p.stats.contextCancel.Load(),
		BytesWrittenTotal:   p.stats.bytesWritten.Load(),
		LastSuccessAt:       formatUnixMilli(p.stats.lastSuccessMS.Load()),
		LastFailureAt:       formatUnixMilli(p.stats.lastFailureMS.Load()),
	}
	p.stats.mu.RLock()
	stats.LastError = p.stats.lastError
	p.stats.mu.RUnlock()
	return stats
}

func (p *Producer) WriteJSON(
	ctx context.Context,
	topic string,
	key string,
	value any,
) error {
	p.stats.writeCalls.Add(1)
	if err := ctx.Err(); err != nil {
		p.recordContextCancel(err)
		return err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		p.recordMarshalFailure(err)
		return err
	}
	payloadBytes := len(payload)
	message := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now().UTC(),
	}
	var lastErr error
	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		startedAt := time.Now()
		writeCtx, cancel := context.WithTimeout(
			ctx,
			p.writeTimeout,
		)
		p.stats.writeAttempts.Add(1)
		err = p.writer.WriteMessages(
			writeCtx,
			message,
		)
		cancel()
		if err == nil {
			p.recordSuccess(
				payloadBytes,
				time.Now(),
			)
			stats := p.Stats()
			p.logInfo(
				"event write succeeded",
				"client_id",
				p.clientID,
				"topic",
				topic,
				"key",
				key,
				"attempt",
				attempt,
				"payload_bytes",
				payloadBytes,
				"duration_ms",
				time.Since(startedAt).Milliseconds(),
				"write_success_total",
				stats.WriteSuccessTotal,
				"write_attempts_total",
				stats.WriteAttemptsTotal,
				"bytes_written_total",
				stats.BytesWrittenTotal,
			)
			return nil
		}
		lastErr = err
		if attempt == p.maxAttempts {
			p.recordFailure(
				err,
				time.Now(),
			)
			stats := p.Stats()
			p.logWarn(
				"event write failed",
				"client_id",
				p.clientID,
				"topic",
				topic,
				"key",
				key,
				"attempt",
				attempt,
				"max_attempts",
				p.maxAttempts,
				"payload_bytes",
				payloadBytes,
				"duration_ms",
				time.Since(startedAt).Milliseconds(),
				"error",
				err,
				"write_failure_total",
				stats.WriteFailureTotal,
				"write_attempts_total",
				stats.WriteAttemptsTotal,
				"write_retry_total",
				stats.WriteRetryTotal,
			)
			break
		}
		p.stats.writeRetry.Add(1)
		p.logWarn(
			"event write retrying",
			"client_id",
			p.clientID,
			"topic",
			topic,
			"key",
			key,
			"attempt",
			attempt,
			"next_attempt",
			attempt+1,
			"max_attempts",
			p.maxAttempts,
			"payload_bytes",
			payloadBytes,
			"duration_ms",
			time.Since(startedAt).Milliseconds(),
			"retry_backoff_ms",
			p.retryBackoff.Milliseconds(),
			"error",
			err,
		)
		timer := time.NewTimer(p.retryBackoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			p.recordContextCancel(ctx.Err())
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func (p *Producer) recordSuccess(
	payloadBytes int,
	at time.Time,
) {
	p.stats.writeSuccess.Add(1)
	p.stats.bytesWritten.Add(uint64(payloadBytes))
	p.stats.lastSuccessMS.Store(at.UnixMilli())
}

func (p *Producer) recordFailure(
	err error,
	at time.Time,
) {
	p.stats.writeFailure.Add(1)
	p.stats.lastFailureMS.Store(at.UnixMilli())
	p.setLastError(err)
}

func (p *Producer) recordMarshalFailure(err error) {
	p.stats.marshalFailure.Add(1)
	p.stats.lastFailureMS.Store(time.Now().UnixMilli())
	p.setLastError(err)
}

func (p *Producer) recordContextCancel(err error) {
	p.stats.contextCancel.Add(1)
	p.stats.lastFailureMS.Store(time.Now().UnixMilli())
	p.setLastError(err)
}

func (p *Producer) setLastError(err error) {
	if err == nil {
		return
	}
	p.stats.mu.Lock()
	p.stats.lastError = err.Error()
	p.stats.mu.Unlock()
}

func formatUnixMilli(value int64) string {
	if value == 0 {
		return ""
	}
	return time.UnixMilli(value).UTC().Format(time.RFC3339Nano)
}

func (p *Producer) logInfo(
	message string,
	args ...any,
) {
	if p.log == nil {
		return
	}
	p.log.Info(
		message,
		args...,
	)
}

func (p *Producer) logWarn(
	message string,
	args ...any,
) {
	if p.log == nil {
		return
	}
	p.log.Warn(
		message,
		args...,
	)
}
