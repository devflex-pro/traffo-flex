package postbacks

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devflex/traffoflex/apps/postback-service/internal/conversions"
	"github.com/devflex/traffoflex/apps/postback-service/internal/outbound"
	"github.com/devflex/traffoflex/apps/postback-service/internal/postbacklogs"
	"github.com/go-chi/chi/v5"
)

func TestHandlerLogsAcceptedPostback(t *testing.T) {
	logs := postbacklogs.NewMemoryLogger()
	handler := NewHandlerWithDeps(
		testLogger(),
		conversions.NewService(conversions.NewMemoryRepository()),
		outbound.NewService(
			nil,
			nil,
		),
		logs,
	)
	router := chi.NewRouter()
	router.Get(
		"/pb/{network}",
		handler.ReceiveGET,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/pb/demo?cid=clk_1&tx=tx_1",
		nil,
	)
	rr := httptest.NewRecorder()
	router.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			http.StatusOK,
		)
	}
	events := logs.Events()
	if len(events) != 1 {
		t.Fatalf(
			"len(events) = %d, want 1",
			len(events),
		)
	}
	if events[0].Status != "accepted" {
		t.Fatalf(
			"Status = %q, want accepted",
			events[0].Status,
		)
	}
}

func TestHandlerLogsRejectedDuplicate(t *testing.T) {
	logs := postbacklogs.NewMemoryLogger()
	handler := NewHandlerWithDeps(
		testLogger(),
		conversions.NewService(conversions.NewMemoryRepository()),
		outbound.NewService(
			nil,
			nil,
		),
		logs,
	)
	router := chi.NewRouter()
	router.Get(
		"/pb/{network}",
		handler.ReceiveGET,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/pb/demo?cid=clk_1&tx=tx_1",
		nil,
	)
	router.ServeHTTP(
		httptest.NewRecorder(),
		req,
	)
	router.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(
			http.MethodGet,
			"/pb/demo?cid=clk_1&tx=tx_1",
			nil,
		),
	)

	events := logs.Events()
	if len(events) != 2 {
		t.Fatalf(
			"len(events) = %d, want 2",
			len(events),
		)
	}
	if events[1].Status != "rejected" {
		t.Fatalf(
			"Status = %q, want rejected",
			events[1].Status,
		)
	}
}

func TestHandlerEnqueuesOutboundPostback(t *testing.T) {
	queue := outbound.NewMemoryQueue(10)
	outboundService := outbound.NewService(queue, []outbound.Template{
		{ID: "tpl_1", Enabled: true, URL: "https://tracker.example/pb?cid={click_id}"},
	})
	handler := NewHandlerWithDeps(
		testLogger(),
		conversions.NewService(conversions.NewMemoryRepository()),
		outboundService,
		postbacklogs.NewMemoryLogger(),
	)
	router := chi.NewRouter()
	router.Get(
		"/pb/{network}",
		handler.ReceiveGET,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/pb/demo?cid=clk_1&tx=tx_1",
		nil,
	)
	rr := httptest.NewRecorder()
	router.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			http.StatusOK,
		)
	}
	if len(queue.Jobs()) != 1 {
		t.Fatalf(
			"len(jobs) = %d, want 1",
			len(queue.Jobs()),
		)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(
		io.Discard,
		nil,
	))
}
