package healthcheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestServiceTrigger(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (
			*http.Response,
			error,
		) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}),
	}

	campaigns := cache.DemoCampaigns()
	campaigns[0].Destinations[0].URL = "https://destination.example/health"
	campaigns[0].Destinations[0].HealthStatus = models.HealthUnknown
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatalf(
			"Reload returned error: %v",
			err,
		)
	}
	service := NewServiceWithClient(
		store,
		nil,
		client,
	)

	result, err := service.Trigger(
		context.Background(),
		"dst_demo",
	)
	if err != nil {
		t.Fatalf(
			"Trigger returned error: %v",
			err,
		)
	}
	if result.DestinationID != "dst_demo" {
		t.Fatalf(
			"DestinationID = %q, want dst_demo",
			result.DestinationID,
		)
	}
	if result.Status != models.HealthHealthy {
		t.Fatalf(
			"Status = %q, want healthy",
			result.Status,
		)
	}
	destination, err := store.GetDestination("dst_demo")
	if err != nil {
		t.Fatalf(
			"GetDestination returned error: %v",
			err,
		)
	}
	if destination.HealthStatus != models.HealthHealthy {
		t.Fatalf(
			"cached HealthStatus = %q, want healthy",
			destination.HealthStatus,
		)
	}
	if result.ScheduledAt.IsZero() {
		t.Fatal("ScheduledAt is zero")
	}
}

type roundTripFunc func(r *http.Request) (
	*http.Response,
	error,
)

func (f roundTripFunc) RoundTrip(r *http.Request) (
	*http.Response,
	error,
) {
	return f(r)
}

func TestServiceTriggerNotFound(t *testing.T) {
	store := cache.NewStore(cache.StaticLoader{Campaigns: cache.DemoCampaigns()})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatalf(
			"Reload returned error: %v",
			err,
		)
	}
	service := NewService(
		store,
		nil,
	)

	_, err := service.Trigger(
		context.Background(),
		"missing",
	)
	if !errors.Is(
		err,
		cache.ErrDestinationNotFound,
	) {
		t.Fatalf(
			"error = %v, want ErrDestinationNotFound",
			err,
		)
	}
}
