package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/api-service/internal/auth"
	"github.com/devflex/traffoflex/apps/api-service/internal/config"
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

func TestClientConfigReturnsTrackerBaseURL(t *testing.T) {
	router := NewRouterWithOptions(
		testLogger(),
		Options{TrackerPublicURL: "https://go.example.com"},
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(
			stdhttp.MethodGet,
			"/api/client-config",
			nil,
		),
	)
	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var payload struct {
		TrackerBaseURL string `json:"tracker_base_url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.TrackerBaseURL != "https://go.example.com" {
		t.Fatalf("tracker URL = %q", payload.TrackerBaseURL)
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

func TestCampaignsRoute(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/campaigns",
		stdhttp.StatusOK,
	)
}

func TestCampaignCRUDRouteFlow(t *testing.T) {
	router := NewRouter(testLogger())

	createBody := []byte(`{"name":"Campaign One","slug":"campaign-one","status":"paused"}`)
	createRR := httptest.NewRecorder()
	router.ServeHTTP(
		createRR,
		httptest.NewRequest(
			stdhttp.MethodPost,
			"/api/campaigns",
			bytes.NewReader(createBody),
		),
	)
	if createRR.Code != stdhttp.StatusCreated {
		t.Fatalf(
			"create status = %d, want %d; body=%s",
			createRR.Code,
			stdhttp.StatusCreated,
			createRR.Body.String(),
		)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(createRR.Body).Decode(&created); err != nil {
		t.Fatalf(
			"decode create response: %v",
			err,
		)
	}
	if created.ID == "" {
		t.Fatal("created id is empty")
	}

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/campaigns/"+created.ID,
		stdhttp.StatusOK,
	)

	updateBody := []byte(`{"name":"Campaign Updated","slug":"campaign-updated","status":"paused"}`)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPut,
		"/api/campaigns/"+created.ID,
		updateBody,
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodDelete,
		"/api/campaigns/"+created.ID,
		stdhttp.StatusNoContent,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/campaigns/"+created.ID,
		stdhttp.StatusNotFound,
	)
}

func TestCampaignCreateRejectsInvalidInput(t *testing.T) {
	router := NewRouter(testLogger())

	body := []byte(`{"name":"Bad","slug":"Bad Slug","status":"active"}`)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPost,
		"/api/campaigns",
		body,
		stdhttp.StatusBadRequest,
	)
}

func TestDestinationCRUDRouteFlow(t *testing.T) {
	router := NewRouter(testLogger())

	createBody := []byte(`{"name":"Destination One","url":"https://example.com/?subid={click_id}"}`)
	createRR := httptest.NewRecorder()
	router.ServeHTTP(
		createRR,
		httptest.NewRequest(
			stdhttp.MethodPost,
			"/api/destinations",
			bytes.NewReader(createBody),
		),
	)
	if createRR.Code != stdhttp.StatusCreated {
		t.Fatalf(
			"create status = %d, want %d; body=%s",
			createRR.Code,
			stdhttp.StatusCreated,
			createRR.Body.String(),
		)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(createRR.Body).Decode(&created); err != nil {
		t.Fatalf(
			"decode create response: %v",
			err,
		)
	}
	if created.ID == "" {
		t.Fatal("created destination id is empty")
	}

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/destinations/"+created.ID,
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodPost,
		"/api/destinations/"+created.ID+"/healthcheck",
		stdhttp.StatusAccepted,
	)

	updateBody := []byte(`{"name":"Destination Updated","url":"https://example.org/?subid={click_id}","manual_status":"paused","health_status":"healthy","redirect":{"mode":"http_302"}}`)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPut,
		"/api/destinations/"+created.ID,
		updateBody,
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodDelete,
		"/api/destinations/"+created.ID,
		stdhttp.StatusNoContent,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/destinations/"+created.ID,
		stdhttp.StatusNotFound,
	)
}

func TestDestinationCreateRejectsUnsafeRedirectOverride(t *testing.T) {
	router := NewRouter(testLogger())

	body := []byte(`{"name":"Unsafe","url":"https://example.com/?redirect_url=https://evil.test"}`)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPost,
		"/api/destinations",
		body,
		stdhttp.StatusBadRequest,
	)
}

func TestStreamCRUDRouteFlow(t *testing.T) {
	router := NewRouter(testLogger())

	createBody := []byte(`{"name":"Primary Stream","priority":10,"distribution":{"mode":"weighted","destinations":[{"destination_id":"dst_1","weight":100}]}}`)
	createRR := httptest.NewRecorder()
	router.ServeHTTP(
		createRR,
		httptest.NewRequest(
			stdhttp.MethodPost,
			"/api/campaigns/cmp_1/streams",
			bytes.NewReader(createBody),
		),
	)
	if createRR.Code != stdhttp.StatusCreated {
		t.Fatalf(
			"create status = %d, want %d; body=%s",
			createRR.Code,
			stdhttp.StatusCreated,
			createRR.Body.String(),
		)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(createRR.Body).Decode(&created); err != nil {
		t.Fatalf(
			"decode create response: %v",
			err,
		)
	}
	if created.ID == "" {
		t.Fatal("created stream id is empty")
	}

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/campaigns/cmp_1/streams",
		stdhttp.StatusOK,
	)

	updateBody := []byte(`{"name":"Updated Stream","priority":20,"status":"paused","distribution":{"mode":"weighted","destinations":[{"destination_id":"dst_1","weight":50}]}}`)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPut,
		"/api/streams/"+created.ID,
		updateBody,
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodDelete,
		"/api/streams/"+created.ID,
		stdhttp.StatusNoContent,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodDelete,
		"/api/streams/"+created.ID,
		stdhttp.StatusNotFound,
	)
}

func TestStreamCreateRejectsInvalidDistribution(t *testing.T) {
	router := NewRouter(testLogger())

	body := []byte(`{"name":"Bad Stream","priority":10,"distribution":{"mode":"weighted","destinations":[{"destination_id":"dst_1","weight":0}]}}`)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPost,
		"/api/campaigns/cmp_1/streams",
		body,
		stdhttp.StatusBadRequest,
	)
}

func TestTrafficSourceCRUDRouteFlow(t *testing.T) {
	router := NewRouter(testLogger())

	createdID := createReferenceResource(
		t,
		router,
		"/api/traffic-sources",
		`{"name":"Meta Ads","slug":"meta-ads"}`,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/traffic-sources/"+createdID,
		stdhttp.StatusOK,
	)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPut,
		"/api/traffic-sources/"+createdID,
		[]byte(`{"name":"Meta Updated","slug":"meta-updated"}`),
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodDelete,
		"/api/traffic-sources/"+createdID,
		stdhttp.StatusNoContent,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/traffic-sources/"+createdID,
		stdhttp.StatusNotFound,
	)
}

func TestAffiliateNetworkCRUDRouteFlow(t *testing.T) {
	router := NewRouter(testLogger())

	createdID := createReferenceResource(
		t,
		router,
		"/api/affiliate-networks",
		`{"name":"Demo Network","slug":"demo-network"}`,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/affiliate-networks/"+createdID,
		stdhttp.StatusOK,
	)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPut,
		"/api/affiliate-networks/"+createdID,
		[]byte(`{"name":"Network Updated","slug":"network-updated"}`),
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodDelete,
		"/api/affiliate-networks/"+createdID,
		stdhttp.StatusNoContent,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/affiliate-networks/"+createdID,
		stdhttp.StatusNotFound,
	)
}

func TestReferenceCreateRejectsInvalidSlug(t *testing.T) {
	router := NewRouter(testLogger())

	body := []byte(`{"name":"Bad","slug":"Bad Slug"}`)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPost,
		"/api/traffic-sources",
		body,
		stdhttp.StatusBadRequest,
	)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPost,
		"/api/affiliate-networks",
		body,
		stdhttp.StatusBadRequest,
	)
}

func TestPostbackTemplateCRUDRouteFlow(t *testing.T) {
	router := NewRouter(testLogger())

	createdID := createReferenceResource(
		t,
		router,
		"/api/postback-templates",
		`{"network_id":"net_1","name":"Default Template","slug":"default-template","mapping":{"click_id":"cid","transaction_id":"tx"}}`,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/postback-templates/"+createdID,
		stdhttp.StatusOK,
	)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPut,
		"/api/postback-templates/"+createdID,
		[]byte(`{"network_id":"net_1","name":"Updated Template","slug":"updated-template","mapping":{"click_id":"subid"}}`),
		stdhttp.StatusOK,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodDelete,
		"/api/postback-templates/"+createdID,
		stdhttp.StatusNoContent,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/postback-templates/"+createdID,
		stdhttp.StatusNotFound,
	)
}

func TestPostbackTemplateCreateRejectsInvalidMapping(t *testing.T) {
	router := NewRouter(testLogger())

	body := []byte(`{"network_id":"net_1","name":"Bad Template","slug":"bad-template","mapping":{"ClickID":"cid"}}`)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPost,
		"/api/postback-templates",
		body,
		stdhttp.StatusBadRequest,
	)
}

func TestReportsRoutes(t *testing.T) {
	router := NewRouter(testLogger())

	for _, path := range []string{
		"/api/reports/overview",
		"/api/reports/campaigns?from=2026-01-01&to=2026-01-31",
		"/api/reports/streams",
		"/api/reports/destinations",
		"/api/reports/sources",
		"/api/reports/trafficback",
		"/api/reports/health",
	} {
		t.Run(
			path,
			func(t *testing.T) {
				assertStatus(
					t,
					router,
					stdhttp.MethodGet,
					path,
					stdhttp.StatusOK,
				)
			},
		)
	}
}

func TestIntegrationTriggerRoutes(t *testing.T) {
	router := NewRouter(testLogger())

	assertStatus(
		t,
		router,
		stdhttp.MethodPost,
		"/api/internal/traffic/cache/reload",
		stdhttp.StatusAccepted,
	)
	assertStatusWithBody(
		t,
		router,
		stdhttp.MethodPost,
		"/api/postbacks/test",
		[]byte(`{"click_id":"clk_1"}`),
		stdhttp.StatusAccepted,
	)
	assertStatus(
		t,
		router,
		stdhttp.MethodPost,
		"/api/destinations/dst_1/healthcheck",
		stdhttp.StatusAccepted,
	)
}

func TestCORSPreflight(t *testing.T) {
	cfg := config.Load()
	router := NewRouterWithConfig(
		testLogger(),
		cfg,
	)
	req := httptest.NewRequest(
		stdhttp.MethodOptions,
		"/api/campaigns",
		nil,
	)
	rr := httptest.NewRecorder()

	router.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != stdhttp.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			stdhttp.StatusNoContent,
		)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf(
			"Access-Control-Allow-Origin = %q",
			got,
		)
	}
}

func TestAuthOTPFlowProtectsAPIRoutes(t *testing.T) {
	router := NewRouterWithOptions(
		testLogger(),
		Options{
			AuthService: auth.NewService(
				auth.NewMemoryRepository(),
				auth.Config{
					AdminEmail:   "admin@example.com",
					JWTSecret:    "test-secret",
					OTPTTL:       10 * time.Minute,
					SessionTTL:   time.Hour,
					DevReturnOTP: true,
				},
			),
		},
	)

	assertStatus(
		t,
		router,
		stdhttp.MethodGet,
		"/api/campaigns",
		stdhttp.StatusUnauthorized,
	)

	otp := requestOTP(
		t,
		router,
		"admin@example.com",
	)
	token := verifyOTP(
		t,
		router,
		"admin@example.com",
		otp,
		stdhttp.StatusOK,
	)
	req := httptest.NewRequest(
		stdhttp.MethodGet,
		"/api/campaigns",
		nil,
	)
	req.Header.Set(
		"Authorization",
		"Bearer "+token,
	)
	rr := httptest.NewRecorder()
	router.ServeHTTP(
		rr,
		req,
	)
	if rr.Code != stdhttp.StatusOK {
		t.Fatalf(
			"authorized campaigns status = %d, want %d; body=%s",
			rr.Code,
			stdhttp.StatusOK,
			rr.Body.String(),
		)
	}
}

func TestAuthPendingUserCannotLoginUntilApproved(t *testing.T) {
	router := NewRouterWithOptions(
		testLogger(),
		Options{
			AuthService: auth.NewService(
				auth.NewMemoryRepository(),
				auth.Config{
					AdminEmail:   "admin@example.com",
					JWTSecret:    "test-secret",
					OTPTTL:       10 * time.Minute,
					SessionTTL:   time.Hour,
					DevReturnOTP: true,
				},
			),
		},
	)

	otp := requestOTP(
		t,
		router,
		"user@example.com",
	)
	verifyOTP(
		t,
		router,
		"user@example.com",
		otp,
		stdhttp.StatusForbidden,
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

func assertStatusWithBody(
	t *testing.T,
	h stdhttp.Handler,
	method,
	path string,
	body []byte,
	want int,
) {
	t.Helper()
	req := httptest.NewRequest(
		method,
		path,
		bytes.NewReader(body),
	)
	rr := httptest.NewRecorder()

	h.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != want {
		t.Fatalf(
			"%s %s status = %d, want %d; body=%s",
			method,
			path,
			rr.Code,
			want,
			rr.Body.String(),
		)
	}
}

func createReferenceResource(
	t *testing.T,
	h stdhttp.Handler,
	path string,
	body string,
) string {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(
		rr,
		httptest.NewRequest(
			stdhttp.MethodPost,
			path,
			bytes.NewReader([]byte(body)),
		),
	)
	if rr.Code != stdhttp.StatusCreated {
		t.Fatalf(
			"POST %s status = %d, want %d; body=%s",
			path,
			rr.Code,
			stdhttp.StatusCreated,
			rr.Body.String(),
		)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf(
			"decode %s response: %v",
			path,
			err,
		)
	}
	if created.ID == "" {
		t.Fatalf(
			"POST %s returned empty id",
			path,
		)
	}
	return created.ID
}

func requestOTP(
	t *testing.T,
	h stdhttp.Handler,
	email string,
) string {
	t.Helper()
	body := []byte(`{"email":"` + email + `"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(
		rr,
		httptest.NewRequest(
			stdhttp.MethodPost,
			"/api/auth/request-otp",
			bytes.NewReader(body),
		),
	)
	if rr.Code != stdhttp.StatusAccepted {
		t.Fatalf(
			"request otp status = %d, want %d; body=%s",
			rr.Code,
			stdhttp.StatusAccepted,
			rr.Body.String(),
		)
	}

	var response struct {
		OTP string `json:"otp"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf(
			"decode otp response: %v",
			err,
		)
	}
	if response.OTP == "" {
		t.Fatal("otp is empty")
	}
	return response.OTP
}

func verifyOTP(
	t *testing.T,
	h stdhttp.Handler,
	email,
	otp string,
	want int,
) string {
	t.Helper()
	body := []byte(`{"email":"` + email + `","otp":"` + otp + `"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(
		rr,
		httptest.NewRequest(
			stdhttp.MethodPost,
			"/api/auth/verify-otp",
			bytes.NewReader(body),
		),
	)
	if rr.Code != want {
		t.Fatalf(
			"verify otp status = %d, want %d; body=%s",
			rr.Code,
			want,
			rr.Body.String(),
		)
	}
	if want != stdhttp.StatusOK {
		return ""
	}

	var response struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf(
			"decode verify response: %v",
			err,
		)
	}
	if response.Token == "" {
		t.Fatal("token is empty")
	}
	return response.Token
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(
		io.Discard,
		nil,
	))
}
