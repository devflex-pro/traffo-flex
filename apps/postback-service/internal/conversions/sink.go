package conversions

import (
	"context"
	"log/slog"
	"sync"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type EventSink interface {
	Write(
		ctx context.Context,
		event models.ConversionEvent,
	) error
}

type BestEffortEventSink struct {
	log  *slog.Logger
	next EventSink
}

func NewBestEffortEventSink(
	log *slog.Logger,
	next EventSink,
) *BestEffortEventSink {
	return &BestEffortEventSink{
		log:  log,
		next: next,
	}
}

func (s *BestEffortEventSink) Write(
	ctx context.Context,
	event models.ConversionEvent,
) error {
	if s.next == nil {
		return nil
	}
	if err := s.next.Write(
		ctx,
		event,
	); err != nil {
		if s.log != nil {
			s.log.Warn(
				"conversion event write failed",
				"error",
				err,
				"conversion_id",
				event.ConversionID,
			)
		}
	}
	return nil
}

type MemoryEventSink struct {
	mu     sync.RWMutex
	events []models.ConversionEvent
}

func NewMemoryEventSink() *MemoryEventSink {
	return &MemoryEventSink{}
}

func (s *MemoryEventSink) Write(
	ctx context.Context,
	event models.ConversionEvent,
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

func (s *MemoryEventSink) Events() []models.ConversionEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make(
		[]models.ConversionEvent,
		len(s.events),
	)
	copy(
		events,
		s.events,
	)
	return events
}
