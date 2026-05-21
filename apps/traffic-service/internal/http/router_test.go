package http

import (
	"errors"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/httpx"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestHealthAndReady(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/healthz",
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/readyz",
		stdhttp.StatusOK,
	)
}

func TestReadyUnavailable(t *testing.T) {
	router := NewRouterWithReadyChecker(testLogger(), httpx.ReadyFunc(func(r *stdhttp.Request) error {
		return errors.New("dependency unavailable")
	}))

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/healthz",
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/readyz",
		stdhttp.StatusServiceUnavailable,
	)
}

func TestEventProducerStatsRoute(t *testing.T) {
	producer, err := eventstream.NewProducer(eventstream.Config{
		Brokers:  []string{"localhost:9092"},
		ClientID: "traffic-test",
	})
	if err != nil {
		t.Fatalf(
			"NewProducer returned error: %v",
			err,
		)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			t.Fatalf(
				"Close returned error: %v",
				err,
			)
		}
	}()
	router := NewRouterWithOptions(
		testLogger(),
		Options{EventProducer: producer},
	)
	req := httptest.NewRequest(
		stdhttp.MethodGet,
		"/internal/event-producer/stats",
		nil,
	)
	rr := httptest.NewRecorder()

	router.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != stdhttp.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			stdhttp.StatusOK,
		)
	}
	if !strings.Contains(
		rr.Body.String(),
		`"client_id":"traffic-test"`,
	) {
		t.Fatalf(
			"unexpected stats body %q",
			rr.Body.String(),
		)
	}
}

func TestCampaignRouteRedirects(t *testing.T) {
	router := NewRouter(testLogger())
	req := httptest.NewRequest(
		stdhttp.MethodGet,
		"/c/demo",
		nil,
	)
	rr := httptest.NewRecorder()

	router.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != stdhttp.StatusFound {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			stdhttp.StatusFound,
		)
	}
	if got := rr.Header().Get("Location"); got == "" || got == "https://example.com/?subid={click_id}" {
		t.Fatalf(
			"unexpected Location header %q",
			got,
		)
	}
}

func TestPublicIDAndTokenRoutesRedirect(t *testing.T) {
	router := NewRouter(testLogger())

	assertRedirect(
		t,
		router,
		"/go/demo-public",
	)
	assertRedirect(
		t,
		router,
		"/r/demo-token",
	)
}

func TestCampaignRouteNoMatchingStream(t *testing.T) {
	store := cache.NewStore(cache.StaticLoader{Campaigns: []cache.CampaignConfig{
		{
			Campaign: models.Campaign{ID: "cmp_1", Slug: "geo", Status: models.StatusActive},
			Streams: []models.Stream{
				{
					ID:     "str_us",
					Status: models.StatusActive,
					Conditions: []models.Condition{
						{Field: "geo_country", Operator: models.OperatorEQ, Value: "US"},
					},
					Distribution: models.Distribution{
						Mode: models.DistributionWeighted,
						Destinations: []models.WeightedTarget{
							{DestinationID: "dst_1", Weight: 100},
						},
					},
				},
			},
			Destinations: []models.Destination{
				{ID: "dst_1", URL: "https://example.com", ManualStatus: models.StatusActive, HealthStatus: models.HealthHealthy},
			},
		},
	}})
	if err := store.Reload(httptest.NewRequest(
		stdhttp.MethodGet,
		"/",
		nil,
	).Context()); err != nil {
		t.Fatalf(
			"Reload returned error: %v",
			err,
		)
	}

	router := NewRouterWithOptions(
		testLogger(),
		Options{Cache: store},
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/c/geo?geo_country=DE",
		stdhttp.StatusNotFound,
	)
}

func TestCampaignRouteNotFound(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/c/missing",
		stdhttp.StatusNotFound,
	)
}

func TestCacheReloadRoute(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodPost,
		"/internal/cache/reload",
		stdhttp.StatusAccepted,
	)
}

func TestDestinationHealthcheckRoute(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodPost,
		"/internal/destinations/dst_demo/healthcheck",
		stdhttp.StatusAccepted,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodPost,
		"/internal/destinations/missing/healthcheck",
		stdhttp.StatusNotFound,
	)
}

func TestTrafficbackRouteRedirectsToConfiguredURL(t *testing.T) {
	router := NewRouter(testLogger())
	req := httptest.NewRequest(
		stdhttp.MethodGet,
		"/tb/demo?click_id=clk_1&reason=no_matching_stream",
		nil,
	)
	rr := httptest.NewRecorder()

	router.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != stdhttp.StatusFound {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			stdhttp.StatusFound,
		)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(
		location,
		"click_id=clk_1",
	) {
		t.Fatalf(
			"Location %q does not include click_id",
			location,
		)
	}
	if !strings.Contains(
		location,
		"depth=1",
	) {
		t.Fatalf(
			"Location %q does not include depth=1",
			location,
		)
	}
}

func TestTrafficbackRouteRejectsLoopDepth(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/tb/demo?trafficback_depth=3",
		stdhttp.StatusTooManyRequests,
	)
}

func assertStatus(
	t *testing.T,
	h stdhttp.Handler,
	method,
	path string,
	want int,
) {
	t.Helper()
	req := httptest.NewRequest(
		method,
		path,
		nil,
	)
	rr := httptest.NewRecorder()

	h.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != want {
		t.Fatalf(
			"%s %s status = %d, want %d",
			method,
			path,
			rr.Code,
			want,
		)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(
		io.Discard,
		nil,
	))
}

func assertRedirect(
	t *testing.T,
	h stdhttp.Handler,
	path string,
) {
	t.Helper()
	req := httptest.NewRequest(
		stdhttp.MethodGet,
		path,
		nil,
	)
	rr := httptest.NewRecorder()

	h.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != stdhttp.StatusFound {
		t.Fatalf(
			"%s status = %d, want %d",
			path,
			rr.Code,
			stdhttp.StatusFound,
		)
	}
	if rr.Header().Get("Location") == "" {
		t.Fatalf(
			"%s Location header is empty",
			path,
		)
	}
}
