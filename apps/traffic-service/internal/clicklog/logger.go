package clicklog

import (
	"context"
	"errors"
	"sync"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var ErrQueueFull = errors.New("click log queue is full")

type Logger interface {
	Log(
		ctx context.Context,
		event models.ClickEvent,
	) error
}

type Sink interface {
	Write(
		ctx context.Context,
		event models.ClickEvent,
	) error
}

type NoopLogger struct{}

func (NoopLogger) Log(
	ctx context.Context,
	event models.ClickEvent,
) error {
	return ctx.Err()
}

type SinkLogger struct {
	sink Sink
}

func NewSinkLogger(sink Sink) *SinkLogger {
	if sink == nil {
		sink = NewMemorySink()
	}
	return &SinkLogger{sink: sink}
}

func (l *SinkLogger) Log(
	ctx context.Context,
	event models.ClickEvent,
) error {
	return l.sink.Write(
		ctx,
		event,
	)
}

type AsyncLogger struct {
	sink  Sink
	queue chan models.ClickEvent
	done  chan struct{}
	once  sync.Once
}

func NewAsyncLogger(
	sink Sink,
	size int,
) *AsyncLogger {
	if sink == nil {
		sink = NewMemorySink()
	}
	if size <= 0 {
		size = 100
	}
	logger := &AsyncLogger{
		sink: sink,
		queue: make(
			chan models.ClickEvent,
			size,
		),
		done: make(chan struct{}),
	}
	go logger.run()
	return logger
}

func (l *AsyncLogger) Log(
	ctx context.Context,
	event models.ClickEvent,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case l.queue <- event:
		return nil
	default:
		return ErrQueueFull
	}
}

func (l *AsyncLogger) Close() {
	l.once.Do(func() {
		close(l.queue)
		<-l.done
	})
}

func (l *AsyncLogger) run() {
	defer close(l.done)
	for event := range l.queue {
		if err := l.sink.Write(
			context.Background(),
			event,
		); err != nil {
			continue
		}
	}
}

type MemorySink struct {
	mu     sync.RWMutex
	events []models.ClickEvent
}

func NewMemorySink() *MemorySink {
	return &MemorySink{}
}

func (s *MemorySink) Write(
	ctx context.Context,
	event models.ClickEvent,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(
		s.events,
		event,
	)
	return nil
}

func (s *MemorySink) Events() []models.ClickEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make(
		[]models.ClickEvent,
		len(s.events),
	)
	copy(
		events,
		s.events,
	)
	return events
}
