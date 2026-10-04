package healthcheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
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

func TestHealthTransitionsRequireConsecutiveResults(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status := http.StatusInternalServerError
		if calls.Add(1) > 3 {
			status = http.StatusNoContent
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	campaigns := cache.DemoCampaigns()
	campaigns[0].Destinations[0].URL = "https://destination.example/offer"
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithThresholds(store, nil, client, nil, Thresholds{Failures: 3, Recovery: 2})
	want := []models.HealthStatus{
		models.HealthDegraded,
		models.HealthDegraded,
		models.HealthUnhealthy,
		models.HealthUnhealthy,
		models.HealthHealthy,
	}
	for index, expected := range want {
		result, err := service.Trigger(context.Background(), "dst_demo")
		if err != nil || result.Status != expected {
			t.Fatalf("probe %d: got %s, %v; want %s", index+1, result.Status, err, expected)
		}
	}
}

func TestNotFoundBecomesUnhealthyAfterThreshold(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	campaigns := cache.DemoCampaigns()
	campaigns[0].Destinations[0].URL = "https://destination.example/offer"
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithThresholds(store, nil, client, nil, Thresholds{Failures: 2, Recovery: 2})
	for index := 0; index < 2; index++ {
		result, err := service.Trigger(context.Background(), "dst_demo")
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 && result.Status != models.HealthDegraded {
			t.Fatalf("first 404 status = %s", result.Status)
		}
		if index == 1 && result.Status != models.HealthUnhealthy {
			t.Fatalf("second 404 status = %s", result.Status)
		}
	}
}

func TestMacroURLNeedsSeparateHealthcheckURL(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	store := cache.NewStore(cache.StaticLoader{Campaigns: cache.DemoCampaigns()})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithClient(store, nil, client)
	result, err := service.Trigger(context.Background(), "dst_demo")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || !strings.Contains(result.Error, "healthcheck_url") {
		t.Fatalf("expected probe to be skipped for macro URL, got %+v, calls=%d", result, calls.Load())
	}
}

func TestGetFallbackUsesOnlyExplicitHealthcheckURL(t *testing.T) {
	var methods []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		methods = append(methods, r.Method+" "+r.URL.String())
		status := http.StatusMethodNotAllowed
		if r.Method == http.MethodGet {
			status = http.StatusNoContent
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	campaigns := cache.DemoCampaigns()
	campaigns[0].Destinations[0].HealthcheckURL = "https://destination.example/health"
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithClient(store, nil, client)
	result, err := service.Trigger(context.Background(), "dst_demo")
	if err != nil || result.Status != models.HealthHealthy {
		t.Fatalf("unexpected probe: %+v, %v", result, err)
	}
	if len(methods) != 2 || methods[0] != "HEAD https://destination.example/health" || methods[1] != "GET https://destination.example/health" {
		t.Fatalf("unexpected probe requests: %v", methods)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://destination.example/error"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})}
	campaigns := cache.DemoCampaigns()
	campaigns[0].Destinations[0].HealthcheckURL = "https://destination.example/health"
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithThresholds(store, nil, client, nil, Thresholds{Failures: 1, Recovery: 2})
	result, err := service.Trigger(context.Background(), "dst_demo")
	if err != nil || result.Status != models.HealthUnhealthy || calls.Load() != 1 {
		t.Fatalf("redirect probe result = %+v, err=%v, calls=%d", result, err, calls.Load())
	}
}

func TestMethodNotAllowedDoesNotGetOffer(t *testing.T) {
	var gets atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			gets.Add(1)
		}
		return &http.Response{StatusCode: http.StatusMethodNotAllowed, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	campaigns := cache.DemoCampaigns()
	campaigns[0].Destinations[0].URL = "https://destination.example/offer"
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithClient(store, nil, client)
	result, err := service.Trigger(context.Background(), "dst_demo")
	if err != nil || gets.Load() != 0 || !strings.Contains(result.Error, "healthcheck_url") {
		t.Fatalf("unsafe GET attempted: %+v, err=%v, gets=%d", result, err, gets.Load())
	}
}

type endlessBody struct{ read int }

func (b *endlessBody) Read(p []byte) (int, error) {
	for index := range p {
		p[index] = 'x'
	}
	b.read += len(p)
	return len(p), nil
}

func (b *endlessBody) Close() error { return nil }

func TestProbeDoesNotDrainUnlimitedBody(t *testing.T) {
	body := &endlessBody{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body, Request: r}, nil
	})}
	campaigns := cache.DemoCampaigns()
	campaigns[0].Destinations[0].HealthcheckURL = "https://destination.example/health"
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithClient(store, nil, client)
	if _, err := service.Trigger(context.Background(), "dst_demo"); err != nil {
		t.Fatal(err)
	}
	if body.read != 4096 {
		t.Fatalf("read %d bytes, want 4096", body.read)
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
