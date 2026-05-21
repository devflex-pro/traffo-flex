package redirects

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestRespondRedirectHTTP302(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/c/demo",
		nil,
	)

	respondRedirect(
		testLogger(),
		rr,
		req,
		"https://example.com",
		models.RedirectHTTP302,
	)

	if rr.Code != http.StatusFound {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			http.StatusFound,
		)
	}
	if rr.Header().Get("Location") != "https://example.com" {
		t.Fatalf(
			"Location = %q",
			rr.Header().Get("Location"),
		)
	}
}

func TestRespondRedirectHTMLModes(t *testing.T) {
	for _, mode := range []models.RedirectMode{models.RedirectMetaRefresh, models.RedirectJavaScript, models.RedirectInterstitial} {
		t.Run(
			string(mode),
			func(t *testing.T) {
				rr := httptest.NewRecorder()
				req := httptest.NewRequest(
					http.MethodGet,
					"/c/demo",
					nil,
				)

				respondRedirect(
					testLogger(),
					rr,
					req,
					"https://example.com",
					mode,
				)

				if rr.Code != http.StatusOK {
					t.Fatalf(
						"status = %d, want %d",
						rr.Code,
						http.StatusOK,
					)
				}
				if !strings.Contains(
					rr.Header().Get("Content-Type"),
					"text/html",
				) {
					t.Fatalf(
						"Content-Type = %q",
						rr.Header().Get("Content-Type"),
					)
				}
			},
		)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(
		io.Discard,
		nil,
	))
}
