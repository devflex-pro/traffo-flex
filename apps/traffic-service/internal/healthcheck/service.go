package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/healthstate"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/trafficevents"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type Result struct {
	DestinationID        string              `json:"destination_id"`
	Status               models.HealthStatus `json:"status"`
	Error                string              `json:"error,omitempty"`
	ScheduledAt          time.Time           `json:"scheduled_at"`
	ProbeStatusCode      int                 `json:"probe_status_code,omitempty"`
	ConsecutiveFailures  int                 `json:"consecutive_failures"`
	ConsecutiveSuccesses int                 `json:"consecutive_successes"`
}

type Thresholds struct {
	Failures int
	Recovery int
}

type probeState struct {
	mu        sync.Mutex
	url       string
	failures  int
	successes int
}

type Service struct {
	cache      *cache.Store
	events     trafficevents.DestinationHealthSink
	client     *http.Client
	repo       healthstate.Repository
	thresholds Thresholds
	states     sync.Map
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
	return NewServiceWithThresholds(store, events, client, repo, Thresholds{Failures: 3, Recovery: 2})
}

func NewServiceWithThresholds(
	store *cache.Store,
	events trafficevents.DestinationHealthSink,
	client *http.Client,
	repo healthstate.Repository,
	thresholds Thresholds,
) *Service {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if thresholds.Failures <= 0 {
		thresholds.Failures = 3
	}
	if thresholds.Recovery <= 0 {
		thresholds.Recovery = 2
	}
	return &Service{
		cache:      store,
		events:     events,
		client:     &clientCopy,
		repo:       repo,
		thresholds: thresholds,
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
	stateValue, _ := s.states.LoadOrStore(destinationID, &probeState{})
	streak := stateValue.(*probeState)
	streak.mu.Lock()
	defer streak.mu.Unlock()
	destination, err := s.cache.GetDestination(destinationID)
	if err != nil {
		return Result{}, err
	}
	now := time.Now().UTC()
	previous := destination.HealthStatus
	probeURL := destination.HealthcheckURL
	explicit := probeURL != ""
	if !explicit {
		probeURL = destination.URL
	}
	if probeURL != streak.url {
		streak.url = probeURL
		streak.failures = 0
		streak.successes = 0
	}
	if strings.ContainsAny(probeURL, "{}") {
		return Result{
			DestinationID: destination.ID,
			Status:        destination.HealthStatus,
			Error:         "healthcheck_url is required for a destination URL with macros",
			ScheduledAt:   now,
		}, nil
	}
	statusCode, errorMessage, err := s.probe(
		ctx,
		probeURL,
		explicit,
	)
	if err != nil {
		return Result{}, err
	}
	current := destination.HealthStatus
	switch {
	case statusCode >= 200 && statusCode < 300 && errorMessage == "":
		streak.successes++
		streak.failures = 0
		if current != models.HealthUnhealthy || streak.successes >= s.thresholds.Recovery {
			current = models.HealthHealthy
		}
	case statusCode == http.StatusMethodNotAllowed && !explicit:
		return Result{
			DestinationID:        destination.ID,
			Status:               current,
			Error:                "HEAD is unsupported; configure healthcheck_url",
			ScheduledAt:          now,
			ProbeStatusCode:      statusCode,
			ConsecutiveFailures:  streak.failures,
			ConsecutiveSuccesses: streak.successes,
		}, nil
	case statusCode >= 300 && statusCode < 400 && !explicit:
		return Result{
			DestinationID:        destination.ID,
			Status:               current,
			Error:                "probe redirected; configure a direct healthcheck_url",
			ScheduledAt:          now,
			ProbeStatusCode:      statusCode,
			ConsecutiveFailures:  streak.failures,
			ConsecutiveSuccesses: streak.successes,
		}, nil
	default:
		if statusCode >= 300 && statusCode < 400 && errorMessage == "" {
			errorMessage = "healthcheck URL redirected; use a direct 2xx URL"
		}
		streak.failures++
		streak.successes = 0
		if streak.failures >= s.thresholds.Failures {
			current = models.HealthUnhealthy
		} else if current != models.HealthUnhealthy {
			current = models.HealthDegraded
		}
	}
	if errorMessage == "" && (statusCode < 200 || statusCode >= 300) {
		errorMessage = fmt.Sprintf("probe returned HTTP %d", statusCode)
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
		OwnerID: destination.OwnerID,
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
				OwnerID: destination.OwnerID,
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
		DestinationID:        destination.ID,
		Status:               destination.HealthStatus,
		Error:                errorMessage,
		ScheduledAt:          now,
		ProbeStatusCode:      statusCode,
		ConsecutiveFailures:  streak.failures,
		ConsecutiveSuccesses: streak.successes,
	}, nil
}

func (s *Service) DestinationIDs() []string {
	return s.cache.DestinationIDs()
}

func (s *Service) probe(
	ctx context.Context,
	url string,
	explicit bool,
) (
	int,
	string,
	error,
) {
	status, errorMessage, err := s.probeMethod(
		ctx,
		http.MethodHead,
		url,
	)
	if err != nil {
		return 0, "", err
	}
	if status != http.StatusMethodNotAllowed || !explicit {
		return status, errorMessage, nil
	}
	status, errorMessage, err = s.probeMethod(
		ctx,
		http.MethodGet,
		url,
	)
	if err != nil {
		return 0, "", err
	}
	return status, errorMessage, nil
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
	if _, drainErr := io.CopyN(
		io.Discard,
		resp.Body,
		4096,
	); drainErr != nil {
		if errors.Is(drainErr, io.EOF) {
			drainErr = nil
		}
		if drainErr == nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
				return resp.StatusCode, closeErr.Error(), nil
			}
			return resp.StatusCode, "", nil
		}
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

func IsNotFound(err error) bool {
	return errors.Is(
		err,
		cache.ErrDestinationNotFound,
	)
}
