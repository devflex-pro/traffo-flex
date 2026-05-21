package postbacktemplates

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type MemoryRepository struct {
	mu        sync.RWMutex
	templates map[string]models.PostbackTemplate
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{templates: make(map[string]models.PostbackTemplate)}
}

func (r *MemoryRepository) List(ctx context.Context) (
	[]models.PostbackTemplate,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	templates := make(
		[]models.PostbackTemplate,
		0,
		len(r.templates),
	)
	for _, template := range r.templates {
		templates = append(
			templates,
			template,
		)
	}
	sort.Slice(
		templates,
		func(
			i,
			j int,
		) bool {
			return templates[i].CreatedAt.Before(templates[j].CreatedAt)
		},
	)
	return templates, nil
}

func (r *MemoryRepository) Get(
	ctx context.Context,
	id string,
) (
	models.PostbackTemplate,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.PostbackTemplate{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	template, ok := r.templates[id]
	if !ok {
		return models.PostbackTemplate{}, ErrNotFound
	}
	return template, nil
}

func (r *MemoryRepository) Create(
	ctx context.Context,
	template models.PostbackTemplate,
) (
	models.PostbackTemplate,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.PostbackTemplate{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.templates[template.ID]; exists {
		return models.PostbackTemplate{}, errors.Join(
			ErrInvalidInput,
			errors.New("postback template id already exists"),
		)
	}
	r.templates[template.ID] = template
	return template, nil
}

func (r *MemoryRepository) Update(
	ctx context.Context,
	template models.PostbackTemplate,
) (
	models.PostbackTemplate,
	error,
) {
	if err := ctx.Err(); err != nil {
		return models.PostbackTemplate{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.templates[template.ID]; !exists {
		return models.PostbackTemplate{}, ErrNotFound
	}
	r.templates[template.ID] = template
	return template, nil
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

	if _, exists := r.templates[id]; !exists {
		return ErrNotFound
	}
	delete(
		r.templates,
		id,
	)
	return nil
}
