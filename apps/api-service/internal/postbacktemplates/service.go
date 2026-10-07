package postbacktemplates

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"github.com/devflex/traffoflex/packages/go-shared/postback"
)

var (
	ErrInvalidInput = errors.New("invalid postback template input")
	ErrNotFound     = errors.New("postback template not found")
)

type Request struct {
	TeamID     string            `json:"team_id,omitempty"`
	NetworkID  string            `json:"network_id"`
	Name       string            `json:"name"`
	Slug       string            `json:"slug"`
	Secret     string            `json:"secret,omitempty"`
	Mapping    map[string]string `json:"mapping"`
	Direction  string            `json:"direction,omitempty"`
	Provider   string            `json:"provider,omitempty"`
	URL        string            `json:"url,omitempty"`
	Enabled    bool              `json:"enabled"`
	SourceID   string            `json:"source_id,omitempty"`
	CampaignID string            `json:"campaign_id,omitempty"`
}

type Repository interface {
	List(ctx context.Context) (
		[]models.PostbackTemplate,
		error,
	)
	Get(
		ctx context.Context,
		id string,
	) (
		models.PostbackTemplate,
		error,
	)
	Create(
		ctx context.Context,
		template models.PostbackTemplate,
	) (
		models.PostbackTemplate,
		error,
	)
	Update(
		ctx context.Context,
		template models.PostbackTemplate,
	) (
		models.PostbackTemplate,
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
	[]models.PostbackTemplate,
	error,
) {
	return s.repo.List(ctx)
}

func (s *Service) Get(
	ctx context.Context,
	id string,
) (
	models.PostbackTemplate,
	error,
) {
	if strings.TrimSpace(id) == "" {
		return models.PostbackTemplate{}, errors.Join(
			ErrInvalidInput,
			errors.New("postback template id is required"),
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
	models.PostbackTemplate,
	error,
) {
	if err := validateRequest(req); err != nil {
		return models.PostbackTemplate{}, err
	}
	now := time.Now().UTC()
	return s.repo.Create(
		ctx,
		models.PostbackTemplate{
			ID:         ids.New("pbt"),
			TeamID:     strings.TrimSpace(req.TeamID),
			NetworkID:  strings.TrimSpace(req.NetworkID),
			Name:       strings.TrimSpace(req.Name),
			Slug:       strings.TrimSpace(req.Slug),
			Secret:     strings.TrimSpace(req.Secret),
			Mapping:    cleanMapping(req.Mapping),
			Direction:  direction(req.Direction),
			Provider:   strings.TrimSpace(req.Provider),
			URL:        strings.TrimSpace(req.URL),
			Enabled:    req.Enabled,
			SourceID:   strings.TrimSpace(req.SourceID),
			CampaignID: strings.TrimSpace(req.CampaignID),
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	)
}

func (s *Service) Update(
	ctx context.Context,
	id string,
	req Request,
) (
	models.PostbackTemplate,
	error,
) {
	if strings.TrimSpace(id) == "" {
		return models.PostbackTemplate{}, errors.Join(
			ErrInvalidInput,
			errors.New("postback template id is required"),
		)
	}
	if err := validateRequest(req); err != nil {
		return models.PostbackTemplate{}, err
	}
	existing, err := s.repo.Get(
		ctx,
		id,
	)
	if err != nil {
		return models.PostbackTemplate{}, err
	}
	if direction(existing.Direction) != direction(req.Direction) {
		return models.PostbackTemplate{}, errors.Join(
			ErrInvalidInput,
			errors.New("postback direction cannot be changed; create a separate integration"),
		)
	}
	existing.TeamID = strings.TrimSpace(req.TeamID)
	existing.NetworkID = strings.TrimSpace(req.NetworkID)
	existing.Name = strings.TrimSpace(req.Name)
	existing.Slug = strings.TrimSpace(req.Slug)
	existing.Secret = strings.TrimSpace(req.Secret)
	existing.Mapping = cleanMapping(req.Mapping)
	existing.Direction = direction(req.Direction)
	existing.Provider = strings.TrimSpace(req.Provider)
	existing.URL = strings.TrimSpace(req.URL)
	existing.Enabled = req.Enabled
	existing.SourceID = strings.TrimSpace(req.SourceID)
	existing.CampaignID = strings.TrimSpace(req.CampaignID)
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
			errors.New("postback template id is required"),
		)
	}
	return s.repo.Delete(
		ctx,
		id,
	)
}

func validateRequest(req Request) error {
	if direction(req.Direction) != "incoming" && direction(req.Direction) != "outgoing" {
		return errors.Join(
			ErrInvalidInput,
			errors.New("invalid postback direction"),
		)
	}
	if direction(req.Direction) == "incoming" && strings.TrimSpace(req.NetworkID) == "" {
		return errors.Join(
			ErrInvalidInput,
			errors.New("network id is required"),
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
	if direction(req.Direction) == "outgoing" {
		if err := postback.ValidateURL(strings.TrimSpace(req.URL)); err != nil {
			return errors.Join(
				ErrInvalidInput,
				err,
			)
		}
		return nil
	}
	if req.Provider != "" && req.Provider != "generic" && req.Provider != "lospollos" {
		return errors.Join(
			ErrInvalidInput,
			errors.New("invalid postback provider"),
		)
	}
	if len(req.Mapping) == 0 {
		return errors.Join(
			ErrInvalidInput,
			errors.New("mapping is required"),
		)
	}
	for key, value := range req.Mapping {
		if err := models.ValidateMacroName(strings.TrimSpace(key)); err != nil {
			return errors.Join(
				ErrInvalidInput,
				err,
			)
		}
		if strings.TrimSpace(value) == "" {
			return errors.Join(
				ErrInvalidInput,
				errors.New("mapping value is required"),
			)
		}
	}
	return nil
}

func direction(value string) string {
	if value == "" {
		return "incoming"
	}
	return value
}

func cleanMapping(mapping map[string]string) map[string]string {
	clean := make(
		map[string]string,
		len(mapping),
	)
	for key, value := range mapping {
		clean[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return clean
}
