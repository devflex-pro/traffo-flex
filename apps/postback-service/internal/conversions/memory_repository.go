package conversions

import (
	"context"
	"sync"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type MemoryRepository struct {
	mu          sync.RWMutex
	conversions map[string]models.ConversionEvent
	dedupe      map[string]struct{}
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		conversions: make(map[string]models.ConversionEvent),
		dedupe:      make(map[string]struct{}),
	}
}

func (r *MemoryRepository) Create(
	ctx context.Context,
	event models.ConversionEvent,
) (
	models.ConversionEvent,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.ConversionEvent{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conversions[event.ConversionID] = event
	return event, nil
}

func (r *MemoryRepository) ExistsDedupe(
	ctx context.Context,
	key DedupeKey,
) (
	bool,
	error,
) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.dedupe[key.String()]
	return ok, nil
}

func (r *MemoryRepository) MarkDedupe(
	ctx context.Context,
	key DedupeKey,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dedupe[key.String()] = struct{}{}
	return nil
}

func (r *MemoryRepository) Events() []models.ConversionEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := make(
		[]models.ConversionEvent,
		0,
		len(r.conversions),
	)
	for _, event := range r.conversions {
		events = append(
			events,
			event,
		)
	}
	return events
}
