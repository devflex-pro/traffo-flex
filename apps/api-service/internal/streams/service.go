package streams

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var (
	ErrInvalidInput = errors.New("invalid stream input")
	ErrNotFound     = errors.New("stream not found")
)

type StreamRequest struct {
	Name         string                   `json:"name"`
	Priority     int                      `json:"priority"`
	Status       models.Status            `json:"status"`
	Conditions   []models.Condition       `json:"conditions"`
	Distribution models.Distribution      `json:"distribution"`
	Trafficback  models.TrafficbackConfig `json:"trafficback_config"`
}

type Repository interface {
	ListByCampaign(
		ctx context.Context,
		campaignID string,
	) (
		[]models.Stream,
		error,
	)
	Get(
		ctx context.Context,
		id string,
	) (
		models.Stream,
		error,
	)
	Create(
		ctx context.Context,
		stream models.Stream,
	) (
		models.Stream,
		error,
	)
	Update(
		ctx context.Context,
		stream models.Stream,
	) (
		models.Stream,
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

func (s *Service) ListByCampaign(
	ctx context.Context,
	campaignID string,
) (
	[]models.Stream,
	error,
) {
	if strings.TrimSpace(campaignID) == "" {
		return nil, errors.Join(
			ErrInvalidInput,
			errors.New("campaign id is required"),
		)
	}
	return s.repo.ListByCampaign(
		ctx,
		campaignID,
	)
}

func (s *Service) Create(
	ctx context.Context,
	campaignID string,
	req StreamRequest,
) (
	models.Stream,
	error,
) {
	campaignID = strings.TrimSpace(campaignID)
	if campaignID == "" {
		return models.Stream{}, errors.Join(
			ErrInvalidInput,
			errors.New("campaign id is required"),
		)
	}
	normalizeDefaults(&req)
	if err := validateRequest(req); err != nil {
		return models.Stream{}, err
	}

	now := time.Now().UTC()
	return s.repo.Create(ctx, models.Stream{
		ID:                ids.New("str"),
		CampaignID:        campaignID,
		Name:              strings.TrimSpace(req.Name),
		Priority:          req.Priority,
		Status:            req.Status,
		Conditions:        req.Conditions,
		Distribution:      req.Distribution,
		TrafficbackConfig: req.Trafficback,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
}

func (s *Service) Update(
	ctx context.Context,
	id string,
	req StreamRequest,
) (
	models.Stream,
	error,
) {
	if strings.TrimSpace(id) == "" {
		return models.Stream{}, errors.Join(
			ErrInvalidInput,
			errors.New("stream id is required"),
		)
	}
	normalizeDefaults(&req)
	if err := validateRequest(req); err != nil {
		return models.Stream{}, err
	}

	existing, err := s.repo.Get(
		ctx,
		id,
	)
	if err != nil {
		return models.Stream{}, err
	}

	existing.Name = strings.TrimSpace(req.Name)
	existing.Priority = req.Priority
	existing.Status = req.Status
	existing.Conditions = req.Conditions
	existing.Distribution = req.Distribution
	existing.TrafficbackConfig = req.Trafficback
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
			errors.New("stream id is required"),
		)
	}
	return s.repo.Delete(
		ctx,
		id,
	)
}

func normalizeDefaults(req *StreamRequest) {
	if req.Status == "" {
		req.Status = models.StatusActive
	}
	if req.Distribution.Mode == "" {
		req.Distribution.Mode = models.DistributionWeighted
	}
}

func validateRequest(req StreamRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.Join(
			ErrInvalidInput,
			errors.New("name is required"),
		)
	}
	if req.Priority < 0 {
		return errors.Join(
			ErrInvalidInput,
			errors.New("priority must be zero or greater"),
		)
	}
	if err := models.ValidateStatus(req.Status); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	if err := models.ValidateDistribution(req.Distribution); err != nil {
		return errors.Join(
			ErrInvalidInput,
			err,
		)
	}
	for _, condition := range req.Conditions {
		if strings.TrimSpace(condition.Field) == "" {
			return errors.Join(
				ErrInvalidInput,
				errors.New("condition field is required"),
			)
		}
		if !condition.Operator.Valid() {
			return errors.Join(
				ErrInvalidInput,
				errors.New("invalid condition operator"),
			)
		}
	}
	return nil
}
