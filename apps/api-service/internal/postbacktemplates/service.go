package postbacktemplates

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var (
	ErrInvalidInput = errors.New("invalid postback template input")
	ErrNotFound     = errors.New("postback template not found")
)

type Request struct {
	TeamID    string            `json:"team_id,omitempty"`
	NetworkID string            `json:"network_id"`
	Name      string            `json:"name"`
	Slug      string            `json:"slug"`
	Secret    string            `json:"secret,omitempty"`
	Mapping   map[string]string `json:"mapping"`
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
	return s.repo.Create(ctx, models.PostbackTemplate{
		ID:        ids.New("pbt"),
		TeamID:    strings.TrimSpace(req.TeamID),
		NetworkID: strings.TrimSpace(req.NetworkID),
		Name:      strings.TrimSpace(req.Name),
		Slug:      strings.TrimSpace(req.Slug),
		Secret:    strings.TrimSpace(req.Secret),
		Mapping:   cleanMapping(req.Mapping),
		CreatedAt: now,
		UpdatedAt: now,
	})
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
	existing.TeamID = strings.TrimSpace(req.TeamID)
	existing.NetworkID = strings.TrimSpace(req.NetworkID)
	existing.Name = strings.TrimSpace(req.Name)
	existing.Slug = strings.TrimSpace(req.Slug)
	existing.Secret = strings.TrimSpace(req.Secret)
	existing.Mapping = cleanMapping(req.Mapping)
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
	if strings.TrimSpace(req.NetworkID) == "" {
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
