package affiliatenetworks

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type MemoryRepository struct {
	mu       sync.RWMutex
	networks map[string]models.AffiliateNetwork
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{networks: make(map[string]models.AffiliateNetwork)}
}

func (r *MemoryRepository) List(ctx context.Context) (
	[]models.AffiliateNetwork,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	networks := make(
		[]models.AffiliateNetwork,
		0,
		len(r.networks),
	)
	for _, network := range r.networks {
		networks = append(
			networks,
			network,
		)
	}
	sort.Slice(
		networks,
		func(
			i,
			j int,
		) bool {
			return networks[i].CreatedAt.Before(networks[j].CreatedAt)
		},
	)
	return networks, nil
}

func (r *MemoryRepository) Get(
	ctx context.Context,
	id string,
) (
	models.AffiliateNetwork,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.AffiliateNetwork{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	network, ok := r.networks[id]
	if !ok {
		return models.AffiliateNetwork{}, ErrNotFound
	}
	return network, nil
}

func (r *MemoryRepository) Create(
	ctx context.Context,
	network models.AffiliateNetwork,
) (
	models.AffiliateNetwork,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.AffiliateNetwork{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.networks[network.ID]; exists {
		return models.AffiliateNetwork{}, errors.Join(
			ErrInvalidInput,
			errors.New("affiliate network id already exists"),
		)
	}
	r.networks[network.ID] = network
	return network, nil
}

func (r *MemoryRepository) Update(
	ctx context.Context,
	network models.AffiliateNetwork,
) (
	models.AffiliateNetwork,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.AffiliateNetwork{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.networks[network.ID]; !exists {
		return models.AffiliateNetwork{}, ErrNotFound
	}
	r.networks[network.ID] = network
	return network, nil
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

	if _, exists := r.networks[id]; !exists {
		return ErrNotFound
	}
	delete(
		r.networks,
		id,
	)
	return nil
}
