package healthcheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/healthstate"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/trafficevents"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type Result struct {
	DestinationID string              `json:"destination_id"`
	Status        models.HealthStatus `json:"status"`
	Error         string              `json:"error,omitempty"`
	ScheduledAt   time.Time           `json:"scheduled_at"`
}

type Service struct {
	cache  *cache.Store
	events trafficevents.DestinationHealthSink
	client *http.Client
	repo   healthstate.Repository
}

func NewService(
	store *cache.Store,
	events trafficevents.DestinationHealthSink,
) *Service {
	return NewServiceWithClient(
		store,
		events,
		nil,
	)
}

func NewServiceWithClient(
	store *cache.Store,
	events trafficevents.DestinationHealthSink,
	client *http.Client,
) *Service {
	return NewServiceWithDependencies(
		store,
		events,
		client,
		nil,
	)
}

func NewServiceWithDependencies(
	store *cache.Store,
	events trafficevents.DestinationHealthSink,
	client *http.Client,
	repo healthstate.Repository,
) *Service {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Service{
		cache:  store,
		events: events,
		client: client,
		repo:   repo,
	}
}

func (s *Service) Trigger(
	ctx context.Context,
	destinationID string,
) (
	Result,
	error,
) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	destination, err := s.cache.GetDestination(destinationID)
	if err != nil {
		return Result{}, err
	}
	now := time.Now().UTC()
	previous := destination.HealthStatus
	current, errorMessage, err := s.probe(
		ctx,
		destination.URL,
	)
	if err != nil {
		return Result{}, err
	}
	destination, err = s.cache.UpdateDestinationHealth(
		destination.ID,
		current,
		now,
	)
	if err != nil {
		return Result{}, err
	}
	state := healthstate.State{
		DestinationID: destination.ID,
		Previous:      previous,
		Current:       current,
		Error:         errorMessage,
		CheckedAt:     now,
		UpdatedAt:     now,
	}
	if s.repo != nil {
		if err := s.repo.Upsert(
			ctx,
			state,
		); err != nil {
			return Result{}, err
		}
	}
	if s.events != nil {
		if err := s.events.WriteDestinationHealth(
			ctx,
			models.DestinationHealthEvent{
				DestinationID: destination.ID,
				Previous:      previous,
				Current:       current,
				Error:         errorMessage,
				CreatedAt:     now,
			},
		); err != nil {
			return Result{}, err
		}
	}
	return Result{
		DestinationID: destination.ID,
		Status:        destination.HealthStatus,
		Error:         errorMessage,
		ScheduledAt:   now,
	}, nil
}

func (s *Service) DestinationIDs() []string {
	return s.cache.DestinationIDs()
}

func (s *Service) probe(
	ctx context.Context,
	url string,
) (
	models.HealthStatus,
	string,
	error,
) {
	status, errorMessage, err := s.probeMethod(
		ctx,
		http.MethodHead,
		url,
	)
	if err != nil {
		return models.HealthUnknown, "", err
	}
	if status != http.StatusMethodNotAllowed {
		return healthFromHTTPStatus(status, errorMessage), errorMessage, nil
	}
	status, errorMessage, err = s.probeMethod(
		ctx,
		http.MethodGet,
		url,
	)
	if err != nil {
		return models.HealthUnknown, "", err
	}
	return healthFromHTTPStatus(status, errorMessage), errorMessage, nil
}

func (s *Service) probeMethod(
	ctx context.Context,
	method string,
	url string,
) (
	int,
	string,
	error,
) {
	if err := ctx.Err(); err != nil {
		return 0, "", err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		method,
		url,
		nil,
	)
	if err != nil {
		return 0, err.Error(), nil
	}
	resp, err := s.client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, "", ctxErr
		}
		return 0, err.Error(), nil
	}
	if _, drainErr := io.Copy(
		io.Discard,
		resp.Body,
	); drainErr != nil {
		closeErr := resp.Body.Close()
		if err := errors.Join(
			drainErr,
			closeErr,
		); err != nil {
			return resp.StatusCode, err.Error(), nil
		}
		return resp.StatusCode, "", nil
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		return resp.StatusCode, closeErr.Error(), nil
	}
	return resp.StatusCode, "", nil
}

func healthFromHTTPStatus(
	status int,
	errorMessage string,
) models.HealthStatus {
	if status == 0 {
		return models.HealthUnhealthy
	}
	if status >= 200 && status < 400 {
		return models.HealthHealthy
	}
	if status >= 400 && status < 500 {
		return models.HealthDegraded
	}
	if errorMessage != "" || status >= 500 {
		return models.HealthUnhealthy
	}
	return models.HealthUnknown
}

func IsNotFound(err error) bool {
	return errors.Is(
		err,
		cache.ErrDestinationNotFound,
	)
}
