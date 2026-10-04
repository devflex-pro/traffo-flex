package conversions

import (
	"context"
	"sync"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type MemoryRepository struct {
	mu                  sync.RWMutex
	conversions         map[string]models.ConversionEvent
	keys                map[string]string
	status              map[string]string
	attributionStatus   map[string]string
	attributionAttempts map[string]int
	attributionNext     map[string]time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		conversions:         make(map[string]models.ConversionEvent),
		keys:                make(map[string]string),
		status:              make(map[string]string),
		attributionStatus:   make(map[string]string),
		attributionAttempts: make(map[string]int),
		attributionNext:     make(map[string]time.Time),
	}
}

func conversionKey(event models.ConversionEvent) string {
	return event.OwnerID + "\x00" + event.NetworkID + "\x00" + event.TransactionID
}

func (r *MemoryRepository) CreateOrGet(
	ctx context.Context,
	event models.ConversionEvent,
) (models.ConversionEvent, bool, error) {
	if err := ctx.Err(); err != nil {
		return models.ConversionEvent{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := conversionKey(event)
	if id, exists := r.keys[key]; exists {
		return r.conversions[id], false, nil
	}
	r.keys[key] = event.ConversionID
	r.conversions[event.ConversionID] = event
	r.status[event.ConversionID] = "pending"
	r.attributionStatus[event.ConversionID] = "pending"
	r.attributionNext[event.ConversionID] = time.Now().UTC()
	return event, true, nil
}

func (r *MemoryRepository) PendingAttribution(
	ctx context.Context,
	limit int,
) ([]AttributionPending, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]AttributionPending, 0)
	for id, event := range r.conversions {
		if r.attributionStatus[id] == "pending" && !r.attributionNext[id].After(time.Now()) {
			result = append(result, AttributionPending{Event: event, Attempts: r.attributionAttempts[id]})
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *MemoryRepository) DeferAttribution(
	ctx context.Context,
	conversionID string,
	attempts int,
	next time.Time,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attributionAttempts[conversionID] = attempts
	r.attributionNext[conversionID] = next
	return nil
}

func (r *MemoryRepository) MarkAttributed(
	ctx context.Context,
	conversionID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attributionStatus[conversionID] = "attributed"
	return nil
}

func (r *MemoryRepository) Pending(
	ctx context.Context,
	limit int,
) ([]models.ConversionEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := make([]models.ConversionEvent, 0)
	for id, event := range r.conversions {
		if r.status[id] == "pending" {
			events = append(events, event)
			if len(events) >= limit {
				break
			}
		}
	}
	return events, nil
}

func (r *MemoryRepository) MarkDelivered(
	ctx context.Context,
	conversionID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status[conversionID] = "delivered"
	return nil
}

func (r *MemoryRepository) Events() []models.ConversionEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := make([]models.ConversionEvent, 0, len(r.conversions))
	for _, event := range r.conversions {
		events = append(events, event)
	}
	return events
}
