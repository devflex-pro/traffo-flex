package campaigns

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var (
	ErrInvalidInput = errors.New("invalid campaign input")
	ErrNotFound     = errors.New("campaign not found")
)

type CampaignRequest struct {
	Name            string        `json:"name"`
	Slug            string        `json:"slug"`
	Status          models.Status `json:"status"`
	TrafficSourceID string        `json:"traffic_source_id,omitempty"`
	Currency        string        `json:"currency,omitempty"`
	DefaultAction   string        `json:"default_action,omitempty"`
}

type Repository interface {
	List(ctx context.Context) (
		[]models.Campaign,
		error,
	)
	Get(
		ctx context.Context,
		id string,
	) (
		models.Campaign,
		error,
	)
	Create(
		ctx context.Context,
		campaign models.Campaign,
	) (
		models.Campaign,
		error,
	)
	Update(
		ctx context.Context,
		campaign models.Campaign,
	) (
		models.Campaign,
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

func (s *Service) SeedDemo() error {
	now := time.Now().UTC()
	_, err := s.repo.Create(context.Background(), models.Campaign{
		ID:        "cmp_demo",
		Name:      "Demo Campaign",
		Slug:      "demo",
		Status:    models.StatusActive,
		Currency:  "USD",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) List(ctx context.Context) (
	[]models.Campaign,
	error,
) {
	return s.repo.List(ctx)
}

func (s *Service) Get(
	ctx context.Context,
	id string,
) (
	models.Campaign,
	error,
) {
	if strings.TrimSpace(id) == "" {
		return models.Campaign{}, errors.Join(
			ErrInvalidInput,
			errors.New("campaign id is required"),
		)
	}
	return s.repo.Get(
		ctx,
		id,
	)
}

func (s *Service) Create(
	ctx context.Context,
	req CampaignRequest,
) (
	models.Campaign,
	error,
) {
	if req.Status == "" {
		req.Status = models.StatusActive
	}
	if strings.TrimSpace(req.Currency) == "" {
		req.Currency = "USD"
	}
	if err := validateRequest(req); err != nil {
		return models.Campaign{}, err
	}

	now := time.Now().UTC()
	return s.repo.Create(ctx, models.Campaign{
		ID:              ids.New("cmp"),
		Name:            strings.TrimSpace(req.Name),
		Slug:            strings.TrimSpace(req.Slug),
		Status:          req.Status,
		TrafficSourceID: strings.TrimSpace(req.TrafficSourceID),
		Currency:        strings.ToUpper(strings.TrimSpace(req.Currency)),
		DefaultAction:   strings.TrimSpace(req.DefaultAction),
		CreatedAt:       now,
		UpdatedAt:       now,
	})
}

func (s *Service) Update(
	ctx context.Context,
	id string,
	req CampaignRequest,
) (
	models.Campaign,
	error,
) {
	if strings.TrimSpace(id) == "" {
		return models.Campaign{}, errors.Join(
			ErrInvalidInput,
			errors.New("campaign id is required"),
		)
	}
	if req.Status == "" {
		req.Status = models.StatusActive
	}
	if strings.TrimSpace(req.Currency) == "" {
		req.Currency = "USD"
	}
	if err := validateRequest(req); err != nil {
		return models.Campaign{}, err
	}

	existing, err := s.repo.Get(
		ctx,
		id,
	)
	if err != nil {
		return models.Campaign{}, err
	}

	existing.Name = strings.TrimSpace(req.Name)
	existing.Slug = strings.TrimSpace(req.Slug)
	existing.Status = req.Status
	existing.TrafficSourceID = strings.TrimSpace(req.TrafficSourceID)
	existing.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	existing.DefaultAction = strings.TrimSpace(req.DefaultAction)
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
			errors.New("campaign id is required"),
		)
	}
	return s.repo.Delete(
		ctx,
		id,
	)
}

func validateRequest(req CampaignRequest) error {
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
	if err := models.ValidateStatus(req.Status); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	if err := models.ValidateCurrency(strings.ToUpper(strings.TrimSpace(req.Currency))); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	return nil
}
