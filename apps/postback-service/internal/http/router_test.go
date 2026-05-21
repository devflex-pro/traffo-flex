package http

import (
	"errors"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/httpx"
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
		ClientID: "postback-test",
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
		`"client_id":"postback-test"`,
	) {
		t.Fatalf(
			"unexpected stats body %q",
			rr.Body.String(),
		)
	}
}

func TestPostbackRoute(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/pb/demo?cid=clk_demo&tx=tx_1",
		stdhttp.StatusOK,
	)
}

func TestPostbackRouteRejectsDuplicate(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/pb/demo?cid=clk_dup&tx=tx_dup",
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/pb/demo?cid=clk_dup&tx=tx_dup",
		stdhttp.StatusConflict,
	)
}

func TestPostbackRouteRejectsMissingTransaction(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/pb/demo?cid=clk_demo",
		stdhttp.StatusBadRequest,
	)
}

func TestPostbackPOSTJSON(t *testing.T) {
	router := NewRouter(testLogger())
	req := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/postbacks",
		strings.NewReader(`{"click_id":"clk_1","transaction_id":"tx_1"}`),
	)
	req.Header.Set(
		"Content-Type",
		"application/json",
	)
	rr := httptest.NewRecorder()

	router.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != stdhttp.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			rr.Code,
			stdhttp.StatusOK,
			rr.Body.String(),
		)
	}
}

func TestInternalRoutes(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodPost,
		"/internal/outbound/retry",
		stdhttp.StatusAccepted,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodPost,
		"/internal/postbacks/test",
		stdhttp.StatusAccepted,
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
