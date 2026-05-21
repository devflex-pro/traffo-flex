package affiliatenetworks

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var (
	ErrInvalidInput = errors.New("invalid affiliate network input")
	ErrNotFound     = errors.New("affiliate network not found")
)

type Request struct {
	TeamID string `json:"team_id,omitempty"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
}

type Repository interface {
	List(ctx context.Context) (
		[]models.AffiliateNetwork,
		error,
	)
	Get(
		ctx context.Context,
		id string,
	) (
		models.AffiliateNetwork,
		error,
	)
	Create(
		ctx context.Context,
		network models.AffiliateNetwork,
	) (
		models.AffiliateNetwork,
		error,
	)
	Update(
		ctx context.Context,
		network models.AffiliateNetwork,
	) (
		models.AffiliateNetwork,
		error,
	)
	Delete(
		ctx context.Context,
		id string,
	) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) (
	[]models.AffiliateNetwork,
	error,
) {
	return s.repo.List(ctx)
}

func (s *Service) Get(
	ctx context.Context,
	id string,
) (
	models.AffiliateNetwork,
	error,
) {
	if strings.TrimSpace(id) == "" {
		return models.AffiliateNetwork{}, errors.Join(
			ErrInvalidInput,
			errors.New("affiliate network id is required"),
		)
	}
	return s.repo.Get(
		ctx,
		id,
	)
}

func (s *Service) Create(
	ctx context.Context,
	req Request,
) (
	models.AffiliateNetwork,
	error,
) {
	if err := validateRequest(req); err != nil {
		return models.AffiliateNetwork{}, err
	}
	now := time.Now().UTC()
	return s.repo.Create(ctx, models.AffiliateNetwork{
		ID:        ids.New("net"),
		TeamID:    strings.TrimSpace(req.TeamID),
		Name:      strings.TrimSpace(req.Name),
		Slug:      strings.TrimSpace(req.Slug),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *Service) Update(
	ctx context.Context,
	id string,
	req Request,
) (
	models.AffiliateNetwork,
	error,
) {
	if strings.TrimSpace(id) == "" {
		return models.AffiliateNetwork{}, errors.Join(
			ErrInvalidInput,
			errors.New("affiliate network id is required"),
		)
	}
	if err := validateRequest(req); err != nil {
		return models.AffiliateNetwork{}, err
	}
	existing, err := s.repo.Get(
		ctx,
		id,
	)
	if err != nil {
		return models.AffiliateNetwork{}, err
	}
	existing.TeamID = strings.TrimSpace(req.TeamID)
	existing.Name = strings.TrimSpace(req.Name)
	existing.Slug = strings.TrimSpace(req.Slug)
	existing.UpdatedAt = time.Now().UTC()
	return s.repo.Update(
		ctx,
		existing,
	)
}

func (s *Service) Delete(
	ctx context.Context,
	id string,
) error {
	if strings.TrimSpace(id) == "" {
		return errors.Join(
			ErrInvalidInput,
			errors.New("affiliate network id is required"),
		)
	}
	return s.repo.Delete(
		ctx,
		id,
	)
}

func validateRequest(req Request) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.Join(
			ErrInvalidInput,
			errors.New("name is required"),
		)
	}
	if err := models.ValidateSlug(strings.TrimSpace(req.Slug)); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	return nil
}
