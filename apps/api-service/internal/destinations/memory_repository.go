package destinations

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type MemoryRepository struct {
	mu           sync.RWMutex
	destinations map[string]models.Destination
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{destinations: make(map[string]models.Destination)}
}

func (r *MemoryRepository) List(ctx context.Context) (
	[]models.Destination,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	destinations := make(
		[]models.Destination,
		0,
		len(r.destinations),
	)
	for _, destination := range r.destinations {
		destinations = append(
			destinations,
			destination,
		)
	}
	sort.Slice(
		destinations,
		func(
			i,
			j int,
		) bool {
			return destinations[i].CreatedAt.Before(destinations[j].CreatedAt)
		},
	)
	return destinations, nil
}

func (r *MemoryRepository) Get(
	ctx context.Context,
	id string,
) (
	models.Destination,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Destination{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	destination, ok := r.destinations[id]
	if !ok {
		return models.Destination{}, ErrNotFound
	}
	return destination, nil
}

func (r *MemoryRepository) Create(
	ctx context.Context,
	destination models.Destination,
) (
	models.Destination,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Destination{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.destinations[destination.ID]; exists {
		return models.Destination{}, errors.Join(
			ErrInvalidInput,
			errors.New("destination id already exists"),
		)
	}
	r.destinations[destination.ID] = destination
	return destination, nil
}

func (r *MemoryRepository) Update(
	ctx context.Context,
	destination models.Destination,
) (
	models.Destination,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.Destination{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.destinations[destination.ID]; !exists {
		return models.Destination{}, ErrNotFound
	}
	r.destinations[destination.ID] = destination
	return destination, nil
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

	if _, exists := r.destinations[id]; !exists {
		return ErrNotFound
	}
	delete(
		r.destinations,
		id,
	)
	return nil
}
