package campaigns

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type MemoryRepository struct {
	mu        sync.RWMutex
	campaigns map[string]models.Campaign
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{campaigns: make(map[string]models.Campaign)}
}

func (r *MemoryRepository) List(ctx context.Context) (
	[]models.Campaign,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	campaigns := make(
		[]models.Campaign,
		0,
		len(r.campaigns),
	)
	for _, campaign := range r.campaigns {
		campaigns = append(
			campaigns,
			campaign,
		)
	}
	sort.Slice(
		campaigns,
		func(
			i,
			j int,
		) bool {
			return campaigns[i].CreatedAt.Before(campaigns[j].CreatedAt)
		},
	)
	return campaigns, nil
}

func (r *MemoryRepository) Get(
	ctx context.Context,
	id string,
) (
	models.Campaign,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Campaign{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	campaign, ok := r.campaigns[id]
	if !ok {
		return models.Campaign{}, ErrNotFound
	}
	return campaign, nil
}

func (r *MemoryRepository) Create(
	ctx context.Context,
	campaign models.Campaign,
) (
	models.Campaign,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Campaign{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.campaigns[campaign.ID]; exists {
		return models.Campaign{}, errors.Join(
			ErrInvalidInput,
			errors.New("campaign id already exists"),
		)
	}
	r.campaigns[campaign.ID] = campaign
	return campaign, nil
}

func (r *MemoryRepository) Update(
	ctx context.Context,
	campaign models.Campaign,
) (
	models.Campaign,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Campaign{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.campaigns[campaign.ID]; !exists {
		return models.Campaign{}, ErrNotFound
	}
	r.campaigns[campaign.ID] = campaign
	return campaign, nil
}

func (r *MemoryRepository) UpdateTrackingParams(
	ctx context.Context,
	id string,
	params []models.TrackingParam,
) (
	models.Campaign,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Campaign{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	campaign, exists := r.campaigns[id]
	if !exists {
		return models.Campaign{}, ErrNotFound
	}
	campaign.TrackingParams = params
	campaign.UpdatedAt = time.Now().UTC()
	r.campaigns[id] = campaign
	return campaign, nil
}

func (r *MemoryRepository) Delete(
	ctx context.Context,
	id string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.campaigns[id]; !exists {
		return ErrNotFound
	}
	delete(
		r.campaigns,
		id,
	)
	return nil
}
