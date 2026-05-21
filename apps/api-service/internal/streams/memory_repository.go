package streams

import (
	"context"
	"sort"
	"sync"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	streams map[string]models.Stream
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{streams: make(map[string]models.Stream)}
}

func (r *MemoryRepository) ListByCampaign(
	ctx context.Context,
	campaignID string,
) (
	[]models.Stream,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	streams := make(
		[]models.Stream,
		0,
	)
	for _, stream := range r.streams {
		if stream.CampaignID == campaignID {
			streams = append(
				streams,
				stream,
			)
		}
	}
	sort.Slice(
		streams,
		func(
			i,
			j int,
		) bool {
			if streams[i].Priority == streams[j].Priority {
				return streams[i].CreatedAt.Before(streams[j].CreatedAt)
			}
			return streams[i].Priority < streams[j].Priority
		},
	)
	return streams, nil
}

func (r *MemoryRepository) Get(
	ctx context.Context,
	id string,
) (
	models.Stream,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Stream{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	stream, ok := r.streams[id]
	if !ok {
		return models.Stream{}, ErrNotFound
	}
	return stream, nil
}

func (r *MemoryRepository) Create(
	ctx context.Context,
	stream models.Stream,
) (
	models.Stream,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Stream{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.streams[stream.ID] = stream
	return stream, nil
}

func (r *MemoryRepository) Update(
	ctx context.Context,
	stream models.Stream,
) (
	models.Stream,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Stream{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.streams[stream.ID]; !exists {
		return models.Stream{}, ErrNotFound
	}
	r.streams[stream.ID] = stream
	return stream, nil
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

	if _, exists := r.streams[id]; !exists {
		return ErrNotFound
	}
	delete(
		r.streams,
		id,
	)
	return nil
}
