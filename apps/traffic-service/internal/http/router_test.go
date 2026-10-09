package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/availability"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/clickaudit"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/clicklog"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/eventqueue"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/healthcheck"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/trafficevents"
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

func TestClickAuditStatsEndpoint(t *testing.T) {
	audit := clickaudit.NewWithDoer(
		"http://clickhouse:8123",
		testLogger(),
		nil,
	)
	router := NewRouterWithOptions(
		testLogger(),
		Options{ClickAudit: audit},
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(stdhttp.MethodGet, "/internal/click-audit/stats", nil),
	)
	if response.Code != stdhttp.StatusOK || !strings.Contains(response.Body.String(), "\"run_total\":0") {
		t.Fatalf("click audit stats response: %d %s", response.Code, response.Body.String())
	}
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

func TestCampaignPricingAppliesToEveryEntrypoint(t *testing.T) {
	campaigns := cache.DemoCampaigns()
	campaigns[0].Campaign.PricingModel = models.PricingCPM
	campaigns[0].Destinations[0].URL = "https://example.com/?cost={cost}"
	campaigns[0].Streams[0].Conditions = []models.Condition{{
		Field:    "cost",
		Operator: models.OperatorEQ,
		Value:    "0.0025",
	}}
	sink := clicklog.NewMemorySink()
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewRouterWithOptions(
		testLogger(),
		Options{
			Cache:       store,
			ClickLogger: clicklog.NewSinkLogger(sink),
		},
	)
	for _, path := range []string{"/c/demo", "/go/demo-public", "/r/demo-token"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(
			response,
			httptest.NewRequest(
				stdhttp.MethodGet,
				path+"?cost=2.5&pricing_model=cpc",
				nil,
			),
		)
		if response.Code != stdhttp.StatusFound || response.Header().Get("Location") != "https://example.com/?cost=0.0025" {
			t.Fatalf(
				"%s returned %d %q",
				path,
				response.Code,
				response.Header().Get("Location"),
			)
		}
	}
	events := sink.Events()
	if len(events) != 3 {
		t.Fatalf(
			"logged events = %d, want 3",
			len(events),
		)
	}
	for _, event := range events {
		if event.Cost != 0.0025 || event.Query["cost"] != "2.5" {
			t.Fatalf(
				"normalized cost/raw price = %v/%q",
				event.Cost,
				event.Query["cost"],
			)
		}
	}
}

func TestRestoredCampaignRedirectsWhileMongoUnavailable(t *testing.T) {
	snapshot := cache.NewSnapshotFile(filepath.Join(t.TempDir(), "campaigns.json"))
	initial := cache.NewPersistentStore(
		cache.StaticLoader{Campaigns: cache.DemoCampaigns()},
		snapshot,
	)
	if err := initial.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	restored := cache.NewPersistentStore(unavailableCampaignLoader{}, snapshot)
	if err := restored.Restore(); err != nil {
		t.Fatal(err)
	}
	if err := restored.Reload(context.Background()); err == nil {
		t.Fatal("expected MongoDB refresh failure")
	}
	router := NewRouterWithOptions(
		testLogger(),
		Options{Cache: restored, ReadyChecker: restored},
	)
	assertRedirect(t, router, "/c/demo")
	assertStatus(t, router, stdhttp.MethodGet, "/readyz", stdhttp.StatusOK)
}

func TestRepeatedDestinationFailureRoutesToTrafficback(t *testing.T) {
	campaigns := cache.DemoCampaigns()
	campaigns[0].Destinations[0].HealthcheckURL = "https://destination.example/health"
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	client := &stdhttp.Client{Transport: healthcheckRoundTripFunc(func(r *stdhttp.Request) (*stdhttp.Response, error) {
		return &stdhttp.Response{
			StatusCode: stdhttp.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})}
	service := healthcheck.NewServiceWithThresholds(
		store,
		nil,
		client,
		nil,
		healthcheck.Thresholds{Failures: 3, Recovery: 2},
	)
	for index := 0; index < 3; index++ {
		if _, err := service.Trigger(context.Background(), "dst_demo"); err != nil {
			t.Fatal(err)
		}
	}
	router := NewRouterWithOptions(testLogger(), Options{Cache: store, HealthService: service})
	request := httptest.NewRequest(stdhttp.MethodGet, "/c/demo", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != stdhttp.StatusFound || !strings.Contains(response.Header().Get("Location"), "/trafficback") {
		t.Fatalf("expected trafficback, got %d %q", response.Code, response.Header().Get("Location"))
	}
}

type healthcheckRoundTripFunc func(*stdhttp.Request) (*stdhttp.Response, error)

func (f healthcheckRoundTripFunc) RoundTrip(request *stdhttp.Request) (*stdhttp.Response, error) {
	return f(request)
}

type unavailableCampaignLoader struct{}

func (unavailableCampaignLoader) Load(ctx context.Context) ([]cache.CampaignConfig, error) {
	return nil, errors.New("mongo unavailable")
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
		stdhttp.StatusServiceUnavailable,
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

func TestCampaignWithoutAvailableDestinationUsesTrafficback(t *testing.T) {
	sink := &recordingTrafficbackSink{}
	router := unavailableDestinationRouter(
		t,
		true,
		sink,
	)
	req := httptest.NewRequest(
		stdhttp.MethodGet,
		"/c/unavailable",
		nil,
	)
	rr := httptest.NewRecorder()
	router.ServeHTTP(
		rr,
		req,
	)
	if rr.Code != stdhttp.StatusFound {
		t.Fatalf(
			"status = %d, want redirect",
			rr.Code,
		)
	}
	location, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf(
			"parse redirect location: %v",
			err,
		)
	}
	if location.Host != "example.org" || location.Path != "/fallback" {
		t.Fatalf(
			"unexpected trafficback location %q",
			location.String(),
		)
	}
	query := location.Query()
	if query.Get("click_id") == "" ||
		query.Get("reason") != string(models.TrafficbackNoDestination) ||
		query.Get("trafficback_depth") != "1" {
		t.Fatalf(
			"unexpected trafficback query %q",
			location.RawQuery,
		)
	}
	if len(sink.events) != 1 ||
		sink.events[0].ClickID != query.Get("click_id") ||
		sink.events[0].Reason != models.TrafficbackNoDestination {
		t.Fatalf(
			"unexpected trafficback events %#v",
			sink.events,
		)
	}
}

func TestMissingCapSnapshotUsesTrafficback(t *testing.T) {
	store := cache.NewStore(cache.StaticLoader{Campaigns: []cache.CampaignConfig{{
		Campaign: models.Campaign{
			ID: "cmp_capped", Slug: "capped", Status: models.StatusActive,
			TrafficbackConfig: models.TrafficbackConfig{
				Enabled: true, URL: "https://example.org/fallback", MaxDepth: 3,
			},
		},
		Streams: []models.Stream{{
			ID: "str_capped", Status: models.StatusActive,
			Distribution: models.Distribution{
				Mode:         models.DistributionDirect,
				Destinations: []models.WeightedTarget{{DestinationID: "dst_capped", Weight: 1}},
			},
		}},
		Destinations: []models.Destination{{
			ID: "dst_capped", URL: "https://example.org/offer",
			ManualStatus: models.StatusActive, HealthStatus: models.HealthHealthy,
			Caps: models.DestinationCaps{
				Enabled: true,
				Rules: []models.DestinationCapRule{{
					Metric: models.CapMetricClicks, WindowHours: 24, Limit: 100,
				}},
			},
		}},
	}}})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot := availability.NewSnapshot(
		store,
		availability.NewClickHouseCapChecker("http://clickhouse:8123"),
		nil,
		time.Second,
	)
	router := NewRouterWithOptions(
		testLogger(),
		Options{
			Cache: store,
			Availability: availability.NewEvaluatorWithSources(
				nil,
				snapshot,
				nil,
				snapshot,
			),
			AvailabilitySnapshot: snapshot,
		},
	)
	request := httptest.NewRequest(stdhttp.MethodGet, "/c/capped", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != stdhttp.StatusFound || response.Header().Get("Location") != "https://example.org/fallback" {
		t.Fatalf("unexpected fallback response: %d %q", response.Code, response.Header().Get("Location"))
	}
	statsResponse := httptest.NewRecorder()
	router.ServeHTTP(
		statsResponse,
		httptest.NewRequest(stdhttp.MethodGet, "/internal/availability/stats", nil),
	)
	if statsResponse.Code != stdhttp.StatusOK ||
		!strings.Contains(statsResponse.Body.String(), "\"missing_cap_checks\":1") {
		t.Fatalf("unexpected availability stats: %d %s", statsResponse.Code, statsResponse.Body.String())
	}
}

func TestRedirectsDoNotWaitForEventDelivery(t *testing.T) {
	clickStarted := make(chan struct{})
	trafficbackStarted := make(chan struct{})
	release := make(chan struct{})
	clickQueue := eventqueue.New[models.ClickEvent](
		"clicks",
		1,
		nil,
		func(ctx context.Context, event models.ClickEvent) error {
			close(clickStarted)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	)
	trafficbackQueue := eventqueue.New[models.TrafficbackEvent](
		"trafficback",
		1,
		nil,
		func(ctx context.Context, event models.TrafficbackEvent) error {
			close(trafficbackStarted)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	)
	clickRouter := NewRouterWithOptions(
		testLogger(),
		Options{
			Cache:            cache.NewDemoStore(),
			ClickLogger:      eventqueue.NewClickLogger(clickQueue),
			ClickQueue:       clickQueue,
			TrafficbackQueue: trafficbackQueue,
		},
	)
	trafficbackRouter := unavailableDestinationRouter(
		t,
		true,
		eventqueue.NewTrafficbackSink(trafficbackQueue),
	)
	assertRedirectReturnsBeforeDelivery(
		t,
		clickRouter,
		"/c/demo",
		clickStarted,
	)
	assertRedirectReturnsBeforeDelivery(
		t,
		trafficbackRouter,
		"/c/unavailable",
		trafficbackStarted,
	)
	close(release)
	if err := clickQueue.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := trafficbackQueue.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if clickQueue.Stats().DeliveredTotal != 1 || trafficbackQueue.Stats().DeliveredTotal != 1 {
		t.Fatalf(
			"events were not delivered after redirects: click=%#v, trafficback=%#v",
			clickQueue.Stats(),
			trafficbackQueue.Stats(),
		)
	}
	statsResponse := httptest.NewRecorder()
	clickRouter.ServeHTTP(
		statsResponse,
		httptest.NewRequest(stdhttp.MethodGet, "/internal/event-queues/stats", nil),
	)
	if statsResponse.Code != stdhttp.StatusOK ||
		!strings.Contains(statsResponse.Body.String(), "\"delivered_total\":1") {
		t.Fatalf(
			"unexpected queue stats response: %d %s",
			statsResponse.Code,
			statsResponse.Body.String(),
		)
	}
}

func TestClickRedirectContinuesWhenEventQueueIsFull(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	queue := eventqueue.New[models.ClickEvent](
		"clicks",
		1,
		nil,
		func(ctx context.Context, event models.ClickEvent) error {
			if event.ClickID != "" {
				select {
				case <-started:
				default:
					close(started)
				}
			}
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	)
	router := NewRouterWithOptions(
		testLogger(),
		Options{
			Cache:       cache.NewDemoStore(),
			ClickLogger: eventqueue.NewClickLogger(queue),
		},
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/c/demo",
		stdhttp.StatusFound,
	)
	<-started
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/c/demo",
		stdhttp.StatusFound,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/c/demo",
		stdhttp.StatusFound,
	)
	if queue.Stats().OverflowTotal != 1 {
		t.Fatalf("overflow count = %d, want 1", queue.Stats().OverflowTotal)
	}
	close(release)
	if err := queue.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestClickRedirectContinuesWhenEventDeliveryFails(t *testing.T) {
	queue := eventqueue.New[models.ClickEvent](
		"clicks",
		1,
		nil,
		func(context.Context, models.ClickEvent) error {
			return errors.New("broker unavailable")
		},
	)
	router := NewRouterWithOptions(
		testLogger(),
		Options{
			Cache:       cache.NewDemoStore(),
			ClickLogger: eventqueue.NewClickLogger(queue),
		},
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/c/demo",
		stdhttp.StatusFound,
	)
	if err := queue.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if queue.Stats().FailedTotal != 1 {
		t.Fatalf(
			"failed delivery count = %d, want 1",
			queue.Stats().FailedTotal,
		)
	}
}

func assertRedirectReturnsBeforeDelivery(
	t *testing.T,
	router stdhttp.Handler,
	path string,
	started <-chan struct{},
) {
	t.Helper()
	done := make(chan int, 1)
	go func() {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, path, nil))
		done <- recorder.Code
	}()
	select {
	case status := <-done:
		if status != stdhttp.StatusFound {
			t.Fatalf("status = %d, want redirect", status)
		}
	case <-time.After(time.Second):
		t.Fatal("redirect waited for event delivery")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("event worker did not start")
	}
}

func TestCampaignWithoutAvailableDestinationAndTrafficbackReturnsError(t *testing.T) {
	router := unavailableDestinationRouter(
		t,
		false,
		nil,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/c/unavailable",
		stdhttp.StatusServiceUnavailable,
	)
}

func TestCampaignWithoutMatchingStreamUsesTrafficback(t *testing.T) {
	sink := &recordingTrafficbackSink{}
	store := cache.NewStore(cache.StaticLoader{Campaigns: []cache.CampaignConfig{
		{
			Campaign: models.Campaign{
				ID:     "cmp_no_stream",
				Slug:   "no-stream",
				Status: models.StatusActive,
				TrafficbackConfig: models.TrafficbackConfig{
					Enabled:  true,
					URL:      "https://example.org/fallback?reason={trafficback_reason}",
					MaxDepth: 3,
				},
			},
		},
	}})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatalf(
			"reload test campaign: %v",
			err,
		)
	}
	router := NewRouterWithOptions(
		testLogger(),
		Options{Cache: store, Trafficback: sink},
	)
	req := httptest.NewRequest(
		stdhttp.MethodGet,
		"/c/no-stream",
		nil,
	)
	rr := httptest.NewRecorder()
	router.ServeHTTP(
		rr,
		req,
	)
	if rr.Code != stdhttp.StatusFound {
		t.Fatalf(
			"status = %d, want redirect",
			rr.Code,
		)
	}
	if len(sink.events) != 1 || sink.events[0].Reason != models.TrafficbackNoMatchingStream {
		t.Fatalf(
			"unexpected trafficback events %#v",
			sink.events,
		)
	}
}

func TestAutomaticTrafficbackRejectsLoopDepth(t *testing.T) {
	sink := &recordingTrafficbackSink{}
	router := unavailableDestinationRouter(
		t,
		true,
		sink,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/c/unavailable?trafficback_depth=3",
		stdhttp.StatusTooManyRequests,
	)
	if len(sink.events) != 0 {
		t.Fatalf(
			"unexpected trafficback events %#v",
			sink.events,
		)
	}
}

func unavailableDestinationRouter(
	t *testing.T,
	trafficbackEnabled bool,
	sink trafficevents.TrafficbackSink,
) stdhttp.Handler {
	t.Helper()
	store := cache.NewStore(cache.StaticLoader{Campaigns: []cache.CampaignConfig{
		{
			Campaign: models.Campaign{
				ID:     "cmp_unavailable",
				Slug:   "unavailable",
				Status: models.StatusActive,
				TrafficbackConfig: models.TrafficbackConfig{
					Enabled:  trafficbackEnabled,
					URL:      "https://example.org/fallback?click_id={click_id}&reason={trafficback_reason}&trafficback_depth={trafficback_depth}",
					MaxDepth: 3,
				},
			},
			Streams: []models.Stream{
				{
					ID:     "str_unavailable",
					Status: models.StatusActive,
					Distribution: models.Distribution{
						Mode: models.DistributionDirect,
						Destinations: []models.WeightedTarget{
							{DestinationID: "dst_unavailable", Weight: 1},
						},
					},
				},
			},
			Destinations: []models.Destination{
				{
					ID:           "dst_unavailable",
					URL:          "https://example.org/offer",
					ManualStatus: models.StatusActive,
					HealthStatus: models.HealthUnhealthy,
				},
			},
		},
	}})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatalf(
			"reload test campaign: %v",
			err,
		)
	}
	return NewRouterWithOptions(
		testLogger(),
		Options{
			Cache:       store,
			Trafficback: sink,
		},
	)
}

type recordingTrafficbackSink struct {
	events []models.TrafficbackEvent
}

func (s *recordingTrafficbackSink) WriteTrafficback(
	ctx context.Context,
	event models.TrafficbackEvent,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.events = append(s.events, event)
	return nil
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
