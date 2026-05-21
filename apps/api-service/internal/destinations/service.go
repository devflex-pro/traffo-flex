package destinations

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var (
	ErrInvalidInput = errors.New("invalid destination input")
	ErrNotFound     = errors.New("destination not found")
)

type DestinationRequest struct {
	Name         string                     `json:"name"`
	Type         models.DestinationType     `json:"type"`
	URL          string                     `json:"url"`
	ManualStatus models.Status              `json:"manual_status"`
	HealthStatus models.HealthStatus        `json:"health_status"`
	Redirect     models.RedirectConfig      `json:"redirect"`
	Trafficback  models.TrafficbackConfig   `json:"trafficback_config"`
	Schedule     models.DestinationSchedule `json:"schedule"`
	Caps         models.DestinationCaps     `json:"caps"`
}

type Repository interface {
	List(ctx context.Context) (
		[]models.Destination,
		error,
	)
	Get(
		ctx context.Context,
		id string,
	) (
		models.Destination,
		error,
	)
	Create(
		ctx context.Context,
		destination models.Destination,
	) (
		models.Destination,
		error,
	)
	Update(
		ctx context.Context,
		destination models.Destination,
	) (
		models.Destination,
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
	_, err := s.repo.Create(context.Background(), models.Destination{
		ID:           "dst_demo",
		Name:         "Demo Destination",
		Type:         models.DestinationURL,
		URL:          "https://example.com/?subid={click_id}",
		ManualStatus: models.StatusActive,
		HealthStatus: models.HealthUnknown,
		Redirect:     models.RedirectConfig{Mode: models.RedirectHTTP302},
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) List(ctx context.Context) (
	[]models.Destination,
	error,
) {
	return s.repo.List(ctx)
}

func (s *Service) Get(
	ctx context.Context,
	id string,
) (
	models.Destination,
	error,
) {
	if strings.TrimSpace(id) == "" {
		return models.Destination{}, errors.Join(
			ErrInvalidInput,
			errors.New("destination id is required"),
		)
	}
	return s.repo.Get(
		ctx,
		id,
	)
}

func (s *Service) Create(
	ctx context.Context,
	req DestinationRequest,
) (
	models.Destination,
	error,
) {
	normalizeDefaults(&req)
	if err := validateRequest(req); err != nil {
		return models.Destination{}, err
	}

	now := time.Now().UTC()
	return s.repo.Create(ctx, models.Destination{
		ID:                ids.New("dst"),
		Name:              strings.TrimSpace(req.Name),
		Type:              req.Type,
		URL:               strings.TrimSpace(req.URL),
		ManualStatus:      req.ManualStatus,
		HealthStatus:      req.HealthStatus,
		Redirect:          req.Redirect,
		TrafficbackConfig: req.Trafficback,
		Schedule:          req.Schedule,
		Caps:              req.Caps,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
}

func (s *Service) Update(
	ctx context.Context,
	id string,
	req DestinationRequest,
) (
	models.Destination,
	error,
) {
	if strings.TrimSpace(id) == "" {
		return models.Destination{}, errors.Join(
			ErrInvalidInput,
			errors.New("destination id is required"),
		)
	}
	normalizeDefaults(&req)
	if err := validateRequest(req); err != nil {
		return models.Destination{}, err
	}

	existing, err := s.repo.Get(
		ctx,
		id,
	)
	if err != nil {
		return models.Destination{}, err
	}

	existing.Name = strings.TrimSpace(req.Name)
	existing.Type = req.Type
	existing.URL = strings.TrimSpace(req.URL)
	existing.ManualStatus = req.ManualStatus
	existing.HealthStatus = req.HealthStatus
	existing.Redirect = req.Redirect
	existing.TrafficbackConfig = req.Trafficback
	existing.Schedule = req.Schedule
	existing.Caps = req.Caps
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
			errors.New("destination id is required"),
		)
	}
	return s.repo.Delete(
		ctx,
		id,
	)
}

func normalizeDefaults(req *DestinationRequest) {
	if req.Type == "" {
		req.Type = models.DestinationURL
	}
	if req.ManualStatus == "" {
		req.ManualStatus = models.StatusActive
	}
	if req.HealthStatus == "" {
		req.HealthStatus = models.HealthUnknown
	}
	if req.Redirect.Mode == "" {
		req.Redirect.Mode = models.RedirectHTTP302
	}
}

func validateRequest(req DestinationRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.Join(
			ErrInvalidInput,
			errors.New("name is required"),
		)
	}
	if !req.Type.Valid() {
		return errors.Join(
			ErrInvalidInput,
			errors.New("invalid destination type"),
		)
	}
	if err := models.ValidateDestinationURL(strings.TrimSpace(req.URL)); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	if err := models.ValidateStatus(req.ManualStatus); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	if err := models.ValidateHealthStatus(req.HealthStatus); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	if !req.Redirect.Mode.Valid() {
		return errors.Join(
			ErrInvalidInput,
			errors.New("invalid redirect mode"),
		)
	}
	if err := models.ValidateDestinationSchedule(req.Schedule); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	if err := models.ValidateDestinationCaps(req.Caps); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	return nil
}
