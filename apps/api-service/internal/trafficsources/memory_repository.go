package trafficsources

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	sources map[string]models.TrafficSource
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{sources: make(map[string]models.TrafficSource)}
}

func (r *MemoryRepository) List(ctx context.Context) (
	[]models.TrafficSource,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	sources := make(
		[]models.TrafficSource,
		0,
		len(r.sources),
	)
	for _, source := range r.sources {
		sources = append(
			sources,
			source,
		)
	}
	sort.Slice(
		sources,
		func(
			i,
			j int,
		) bool {
			return sources[i].CreatedAt.Before(sources[j].CreatedAt)
		},
	)
	return sources, nil
}

func (r *MemoryRepository) Get(
	ctx context.Context,
	id string,
) (
	models.TrafficSource,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.TrafficSource{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	source, ok := r.sources[id]
	if !ok {
		return models.TrafficSource{}, ErrNotFound
	}
	return source, nil
}

func (r *MemoryRepository) Create(
	ctx context.Context,
	source models.TrafficSource,
) (
	models.TrafficSource,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.TrafficSource{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sources[source.ID]; exists {
		return models.TrafficSource{}, errors.Join(
			ErrInvalidInput,
			errors.New("traffic source id already exists"),
		)
	}
	r.sources[source.ID] = source
	return source, nil
}

func (r *MemoryRepository) Update(
	ctx context.Context,
	source models.TrafficSource,
) (
	models.TrafficSource,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.TrafficSource{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sources[source.ID]; !exists {
		return models.TrafficSource{}, ErrNotFound
	}
	r.sources[source.ID] = source
	return source, nil
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

	if _, exists := r.sources[id]; !exists {
		return ErrNotFound
	}
	delete(
		r.sources,
		id,
	)
	return nil
}
