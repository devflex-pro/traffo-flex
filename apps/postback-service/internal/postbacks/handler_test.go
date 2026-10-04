package postbacks

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devflex/traffoflex/apps/postback-service/internal/conversions"
	"github.com/devflex/traffoflex/apps/postback-service/internal/normalize"
	"github.com/devflex/traffoflex/apps/postback-service/internal/outbound"
	"github.com/devflex/traffoflex/apps/postback-service/internal/postbacklogs"
	"github.com/go-chi/chi/v5"
)

type testSecretStore struct{}

type testOwnerSecretStore struct{ testSecretStore }

func (testOwnerSecretStore) Credentials(
	ctx context.Context,
	networkID string,
) ([]normalize.Credential, error) {
	return []normalize.Credential{{OwnerID: "usr_1", Secret: "valid"}}, nil
}

func (testSecretStore) Secrets(
	ctx context.Context,
	networkID string,
) ([]string, error) {
	return []string{"test-secret"}, nil
}

func TestHandlerLogsAcceptedPostback(t *testing.T) {
	logs := postbacklogs.NewMemoryLogger()
	handler := NewHandlerWithSecrets(
		testLogger(),
		conversions.NewService(conversions.NewMemoryRepository()),
		outbound.NewService(
			nil,
			nil,
		),
		logs,
		testSecretStore{},
	)
	router := chi.NewRouter()
	router.Get(
		"/pb/{network}",
		handler.ReceiveGET,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/pb/demo?cid=clk_1&tx=tx_1&secret=test-secret",
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

func TestHandlerAttributesRejectedPostbackToKnownNetworkOwner(t *testing.T) {
	logs := postbacklogs.NewMemoryLogger()
	handler := NewHandlerWithSecrets(
		testLogger(),
		conversions.NewService(conversions.NewMemoryRepository()),
		outbound.NewService(
			nil,
			nil,
		),
		logs,
		testOwnerSecretStore{},
	)
	router := chi.NewRouter()
	router.Get(
		"/pb/{network}",
		handler.ReceiveGET,
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/pb/net_1?cid=clk_1&tx=tx_1&secret=invalid",
			nil,
		),
	)
	events := logs.Events()
	if response.Code != http.StatusUnauthorized || len(events) != 1 || events[0].OwnerID != "usr_1" {
		t.Fatalf(
			"status = %d, events = %+v",
			response.Code,
			events,
		)
	}
}

func TestHandlerLogsAcceptedDuplicate(t *testing.T) {
	logs := postbacklogs.NewMemoryLogger()
	handler := NewHandlerWithSecrets(
		testLogger(),
		conversions.NewService(conversions.NewMemoryRepository()),
		outbound.NewService(
			nil,
			nil,
		),
		logs,
		testSecretStore{},
	)
	router := chi.NewRouter()
	router.Get(
		"/pb/{network}",
		handler.ReceiveGET,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/pb/demo?cid=clk_1&tx=tx_1&secret=test-secret",
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
			"/pb/demo?cid=clk_1&tx=tx_1&secret=test-secret",
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
	if events[1].Status != "duplicate" {
		t.Fatalf(
			"Status = %q, want duplicate",
			events[1].Status,
		)
	}
}

func TestHandlerEnqueuesOutboundPostback(t *testing.T) {
	queue := outbound.NewMemoryQueue(10)
	outboundService := outbound.NewService(queue, []outbound.Template{
		{ID: "tpl_1", Enabled: true, URL: "https://tracker.example/pb?cid={click_id}"},
	})
	handler := NewHandlerWithSecrets(
		testLogger(),
		conversions.NewService(conversions.NewMemoryRepository()),
		outboundService,
		postbacklogs.NewMemoryLogger(),
		testSecretStore{},
	)
	router := chi.NewRouter()
	router.Get(
		"/pb/{network}",
		handler.ReceiveGET,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/pb/demo?cid=clk_1&tx=tx_1&secret=test-secret",
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
