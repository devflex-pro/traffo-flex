package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONWritesStatusAndBody(t *testing.T) {
	rr := httptest.NewRecorder()

	if err := JSON(
		rr,
		http.StatusCreated,
		map[string]string{"status": "ok"},
	); err != nil {
		t.Fatalf(
			"JSON returned error: %v",
			err,
		)
	}

	if rr.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			http.StatusCreated,
		)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}
	if body["status"] != "ok" {
		t.Fatalf(
			"body status = %q, want ok",
			body["status"],
		)
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := CORS("http://localhost:5173")(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		t.Fatal("next handler should not be called for preflight")
	}))
	req := httptest.NewRequest(
		http.MethodOptions,
		"/",
		nil,
	)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(
		rr,
		req,
	)

	if rr.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			http.StatusNoContent,
		)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf(
			"Access-Control-Allow-Origin = %q",
			got,
		)
	}
}
