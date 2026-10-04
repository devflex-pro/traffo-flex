package integrations

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPClientReloadTrafficCache(t *testing.T) {
	var path string
	doer := fakeDoer{fn: func(req *http.Request) (
		*http.Response,
		error,
	) {
		path = req.URL.Path
		return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(http.NoBody)}, nil
	}}

	client := NewHTTPClientWithDoer(
		"http://traffic-service:8080",
		"http://postback-service:8081",
		doer,
	)
	if err := client.ReloadTrafficCache(context.Background()); err != nil {
		t.Fatalf(
			"ReloadTrafficCache returned error: %v",
			err,
		)
	}
	if path != "/internal/cache/reload" {
		t.Fatalf(
			"path = %q, want /internal/cache/reload",
			path,
		)
	}
}

func TestHTTPClientReturnsErrorForNon2xx(t *testing.T) {
	doer := fakeDoer{fn: func(req *http.Request) (
		*http.Response,
		error,
	) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(http.NoBody)}, nil
	}}

	client := NewHTTPClientWithDoer(
		"http://traffic-service:8080",
		"http://postback-service:8081",
		doer,
	)
	if err := client.ReloadTrafficCache(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPClientReturnsHealthcheckResult(t *testing.T) {
	doer := fakeDoer{fn: func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/internal/destinations/dst_1/healthcheck" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader(`{"destination_id":"dst_1","status":"degraded","probe_status_code":404,"consecutive_failures":1}`)),
		}, nil
	}}
	client := NewHTTPClientWithDoer("http://traffic-service:8080", "http://postback-service:8081", doer)
	result, err := client.TriggerDestinationHealthcheck(context.Background(), "dst_1")
	if err != nil || result.Status != "degraded" || result.ConsecutiveFailures != 1 {
		t.Fatalf("unexpected result: %+v, %v", result, err)
	}
}

type fakeDoer struct {
	fn func(req *http.Request) (
		*http.Response,
		error,
	)
}

func (d fakeDoer) Do(req *http.Request) (
	*http.Response,
	error,
) {
	return d.fn(req)
}
