package eventqueue

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrFull   = errors.New("event queue is full")
	ErrClosed = errors.New("event queue is closed")
)

type WriteFunc[T any] func(context.Context, T) error
type WriteBatchFunc[T any] func(context.Context, []T) error

type Stats struct {
	Capacity        int    `json:"capacity"`
	Pending         int64  `json:"pending"`
	AcceptedTotal   uint64 `json:"accepted_total"`
	DeliveredTotal  uint64 `json:"delivered_total"`
	FailedTotal     uint64 `json:"failed_total"`
	OverflowTotal   uint64 `json:"overflow_total"`
	StoppedTotal    uint64 `json:"stopped_total"`
	Closed          bool   `json:"closed"`
	DiskBytes       int64  `json:"disk_bytes,omitempty"`
	MaxDiskBytes    int64  `json:"max_disk_bytes,omitempty"`
	RetryTotal      uint64 `json:"retry_total,omitempty"`
	RecoveredTotal  int64  `json:"recovered_total,omitempty"`
	RecoveryPending bool   `json:"recovery_pending,omitempty"`
}

type Queue[T any] struct {
	name       string
	log        *slog.Logger
	writeBatch WriteBatchFunc[T]
	batchSize  int
	batchDelay time.Duration
	items      chan T
	mu         sync.Mutex
	closed     bool
	done       chan struct{}
	workerCtx  context.Context
	cancel     context.CancelFunc
	pending    atomic.Int64
	accepted   atomic.Uint64
	delivered  atomic.Uint64
	failed     atomic.Uint64
	overflow   atomic.Uint64
	stopped    atomic.Uint64
}

func New[T any](
	name string,
	capacity int,
	log *slog.Logger,
	write WriteFunc[T],
) *Queue[T] {
	return NewBatched(
		name,
		capacity,
		1,
		0,
		log,
		func(ctx context.Context, events []T) error {
			return write(ctx, events[0])
		},
	)
}

func NewBatched[T any](
	name string,
	capacity int,
	batchSize int,
	batchDelay time.Duration,
	log *slog.Logger,
	write WriteBatchFunc[T],
) *Queue[T] {
	if capacity <= 0 {
		capacity = 1000
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	workerCtx, cancel := context.WithCancel(context.Background())
	q := &Queue[T]{
		name:       name,
		log:        log,
		writeBatch: write,
		batchSize:  batchSize,
		batchDelay: batchDelay,
		items:      make(chan T, capacity),
		done:       make(chan struct{}),
		workerCtx:  workerCtx,
		cancel:     cancel,
	}
	go q.run()
	return q
}

func (q *Queue[T]) Submit(
	ctx context.Context,
	event T,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrClosed
	}
	q.pending.Add(1)
	select {
	case q.items <- event:
		q.accepted.Add(1)
		q.mu.Unlock()
		return nil
	default:
		q.pending.Add(-1)
		count := q.overflow.Add(1)
		q.mu.Unlock()
		if count == 1 || count%1000 == 0 {
			q.warn("event queue overflow", "overflow_total", count)
		}
		return ErrFull
	}
}

func (q *Queue[T]) Stats() Stats {
	q.mu.Lock()
	closed := q.closed
	q.mu.Unlock()
	return Stats{
		Capacity:       cap(q.items),
		Pending:        q.pending.Load(),
		AcceptedTotal:  q.accepted.Load(),
		DeliveredTotal: q.delivered.Load(),
		FailedTotal:    q.failed.Load(),
		OverflowTotal:  q.overflow.Load(),
		StoppedTotal:   q.stopped.Load(),
		Closed:         closed,
	}
}

func (q *Queue[T]) Close(ctx context.Context) error {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.items)
	}
	q.mu.Unlock()
	select {
	case <-q.done:
		return nil
	case <-ctx.Done():
		q.cancel()
		return ctx.Err()
	}
}

func (q *Queue[T]) Done() <-chan struct{} {
	return q.done
}

func (q *Queue[T]) run() {
	defer close(q.done)
	for event := range q.items {
		batch := []T{event}
		closed := false
		if q.batchSize > 1 {
			var timer *time.Timer
			if q.batchDelay > 0 {
				timer = time.NewTimer(q.batchDelay)
			}
		gather:
			for len(batch) < q.batchSize {
				if timer == nil {
					select {
					case next, ok := <-q.items:
						if !ok {
							closed = true
							break gather
						}
						batch = append(batch, next)
					default:
						break gather
					}
					continue
				}
				select {
				case next, ok := <-q.items:
					if !ok {
						closed = true
						break gather
					}
					batch = append(batch, next)
				case <-timer.C:
					break gather
				case <-q.workerCtx.Done():
					break gather
				}
			}
			if timer != nil {
				timer.Stop()
			}
		}
		count := uint64(len(batch))
		if q.workerCtx.Err() != nil {
			q.stopped.Add(count)
		} else {
			if err := q.writeBatch(q.workerCtx, batch); err != nil {
				failed := q.failed.Add(count)
				if failed == count || failed%1000 == 0 {
					q.warn("event delivery failed", "failed_total", failed, "error", err)
				}
			} else {
				q.delivered.Add(count)
			}
		}
		q.pending.Add(-int64(count))
		if closed {
			break
		}
	}
	q.cancel()
}

func (q *Queue[T]) warn(message string, args ...any) {
	if q.log == nil {
		return
	}
	q.log.Warn(message, append([]any{"queue", q.name}, args...)...)
}
