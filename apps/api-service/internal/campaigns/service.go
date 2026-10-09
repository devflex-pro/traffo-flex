package campaigns

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var (
	ErrInvalidInput         = errors.New("invalid campaign input")
	ErrNotFound             = errors.New("campaign not found")
	trackingParamKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

type CampaignRequest struct {
	Name            string                    `json:"name"`
	Slug            string                    `json:"slug"`
	Status          models.Status             `json:"status"`
	TrafficSourceID string                    `json:"traffic_source_id,omitempty"`
	Currency        string                    `json:"currency,omitempty"`
	PricingModel    models.PricingModel       `json:"pricing_model,omitempty"`
	DefaultAction   string                    `json:"default_action,omitempty"`
	Trafficback     *models.TrafficbackConfig `json:"trafficback_config,omitempty"`
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
	UpdateTrackingParams(
		ctx context.Context,
		id string,
		params []models.TrackingParam,
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
	if req.PricingModel == "" {
		req.PricingModel = models.PricingCPC
	}
	if err := validateRequest(req); err != nil {
		return models.Campaign{}, err
	}
	trafficback := normalizeTrafficback(req.Trafficback)
	if err := validateActiveTrafficback(
		req.Status,
		trafficback,
	); err != nil {
		return models.Campaign{}, err
	}

	now := time.Now().UTC()
	return s.repo.Create(ctx, models.Campaign{
		ID:                ids.New("cmp"),
		Name:              strings.TrimSpace(req.Name),
		Slug:              strings.TrimSpace(req.Slug),
		Status:            req.Status,
		TrafficSourceID:   strings.TrimSpace(req.TrafficSourceID),
		Currency:          strings.ToUpper(strings.TrimSpace(req.Currency)),
		PricingModel:      req.PricingModel,
		DefaultAction:     strings.TrimSpace(req.DefaultAction),
		TrafficbackConfig: trafficback,
		CreatedAt:         now,
		UpdatedAt:         now,
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
	if req.PricingModel != "" {
		existing.PricingModel = req.PricingModel
	}
	existing.DefaultAction = strings.TrimSpace(req.DefaultAction)
	if req.Trafficback != nil {
		existing.TrafficbackConfig = normalizeTrafficback(req.Trafficback)
	}
	if err := validateActiveTrafficback(
		existing.Status,
		existing.TrafficbackConfig,
	); err != nil {
		return models.Campaign{}, err
	}
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
	campaign, err := s.repo.Get(
		ctx,
		id,
	)
	if err != nil {
		return err
	}
	if campaign.Status != models.StatusArchived {
		return errors.Join(
			ErrInvalidInput,
			errors.New("campaign must be archived before deletion"),
		)
	}
	return s.repo.Delete(
		ctx,
		id,
	)
}

func (s *Service) UpdateTrackingParams(
	ctx context.Context,
	id string,
	params []models.TrackingParam,
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
	if err := validateTrackingParams(params); err != nil {
		return models.Campaign{}, err
	}
	normalized := make(
		[]models.TrackingParam,
		0,
		len(params),
	)
	for _, param := range params {
		normalized = append(
			normalized,
			models.TrackingParam{
				Key:   strings.TrimSpace(param.Key),
				Value: strings.TrimSpace(param.Value),
			},
		)
	}
	return s.repo.UpdateTrackingParams(
		ctx,
		id,
		normalized,
	)
}

func validateTrackingParams(params []models.TrackingParam) error {
	if len(params) > 24 {
		return errors.Join(
			ErrInvalidInput,
			errors.New("too many tracking parameters"),
		)
	}
	seen := make(
		map[string]struct{},
		len(params),
	)
	for _, param := range params {
		key := strings.TrimSpace(param.Key)
		value := strings.TrimSpace(param.Value)
		if !trackingParamKeyPattern.MatchString(key) || value == "" || len(value) > 256 {
			return errors.Join(
				ErrInvalidInput,
				errors.New("invalid tracking parameter"),
			)
		}
		if _, exists := seen[key]; exists {
			return errors.Join(
				ErrInvalidInput,
				errors.New("duplicate tracking parameter"),
			)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateRequest(req CampaignRequest) error {
	if req.PricingModel != "" && !req.PricingModel.Valid() {
		return errors.Join(
			ErrInvalidInput,
			errors.New("pricing model must be cpc or cpm"),
		)
	}
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
	if req.Trafficback != nil && req.Trafficback.Enabled {
		parsed, err := url.Parse(strings.TrimSpace(req.Trafficback.URL))
		if err != nil || parsed == nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Hostname() == "" || parsed.User != nil {
			return errors.Join(
				ErrInvalidInput,
				errors.New("trafficback URL must be an absolute HTTP or HTTPS URL"),
			)
		}
		if req.Trafficback.MaxDepth < 1 || req.Trafficback.MaxDepth > 10 {
			return errors.Join(
				ErrInvalidInput,
				errors.New("trafficback max depth must be between 1 and 10"),
			)
		}
	}
	return nil
}

func normalizeTrafficback(config *models.TrafficbackConfig) models.TrafficbackConfig {
	if config == nil || !config.Enabled {
		return models.TrafficbackConfig{}
	}
	result := *config
	result.URL = strings.TrimSpace(result.URL)
	return result
}

func validateActiveTrafficback(
	status models.Status,
	config models.TrafficbackConfig,
) error {
	if status == models.StatusActive && (!config.Enabled || config.URL == "") {
		return errors.Join(
			ErrInvalidInput,
			errors.New("active campaign requires a trafficback URL"),
		)
	}
	return nil
}
