package postbacklogs

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type Logger interface {
	Log(
		ctx context.Context,
		event models.PostbackLogEvent,
	) error
}

type BestEffortLogger struct {
	log  *slog.Logger
	next Logger
}

func NewBestEffortLogger(
	log *slog.Logger,
	next Logger,
) *BestEffortLogger {
	return &BestEffortLogger{
		log:  log,
		next: next,
	}
}

func (l *BestEffortLogger) Log(
	ctx context.Context,
	event models.PostbackLogEvent,
) error {
	if l.next == nil {
		return nil
	}
	if err := l.next.Log(
		ctx,
		event,
	); err != nil {
		if l.log != nil {
			l.log.Warn(
				"postback log event write failed",
				"error",
				err,
				"postback_id",
				event.PostbackID,
			)
		}
	}
	return nil
}

type MultiLogger struct {
	loggers []Logger
}

func NewMultiLogger(loggers ...Logger) *MultiLogger {
	items := make(
		[]Logger,
		0,
		len(loggers),
	)
	for _, logger := range loggers {
		if logger != nil {
			items = append(
				items,
				logger,
			)
		}
	}
	return &MultiLogger{loggers: items}
}

func (l *MultiLogger) Log(
	ctx context.Context,
	event models.PostbackLogEvent,
) error {
	errs := make(
		[]error,
		0,
		len(l.loggers),
	)
	for _, logger := range l.loggers {
		if err := logger.Log(
			ctx,
			event,
		); err != nil {
			errs = append(
				errs,
				err,
			)
		}
	}
	return errors.Join(errs...)
}

type MemoryLogger struct {
	mu     sync.RWMutex
	events []models.PostbackLogEvent
}

func NewMemoryLogger() *MemoryLogger {
	return &MemoryLogger{}
}

func (l *MemoryLogger) Log(
	ctx context.Context,
	event models.PostbackLogEvent,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(
		l.events,
		event,
	)
	return nil
}

func (l *MemoryLogger) Events() []models.PostbackLogEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()
	events := make(
		[]models.PostbackLogEvent,
		len(l.events),
	)
	copy(
		events,
		l.events,
	)
	return events
}
