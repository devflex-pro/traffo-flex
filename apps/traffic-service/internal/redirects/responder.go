package redirects

import (
	"html"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func respondRedirect(
	log *slog.Logger,
	w http.ResponseWriter,
	r *http.Request,
	targetURL string,
	mode models.RedirectMode,
) {
	switch mode {
	case "", models.RedirectHTTP302:
		http.Redirect(
			w,
			r,
			targetURL,
			http.StatusFound,
		)
	case models.RedirectMetaRefresh:
		writeHTML(
			log,
			w,
			http.StatusOK,
			`<!doctype html><html><head><meta http-equiv="refresh" content="0;url=`+html.EscapeString(targetURL)+`"></head><body></body></html>`,
		)
	case models.RedirectJavaScript:
		writeHTML(
			log,
			w,
			http.StatusOK,
			`<!doctype html><html><body><script>window.location.replace(`+strconv.Quote(targetURL)+`);</script></body></html>`,
		)
	case models.RedirectInterstitial:
		writeHTML(
			log,
			w,
			http.StatusOK,
			`<!doctype html><html><body><a rel="nofollow" href="`+html.EscapeString(targetURL)+`">Continue</a></body></html>`,
		)
	default:
		http.Redirect(
			w,
			r,
			targetURL,
			http.StatusFound,
		)
	}
}

func writeHTML(
	log *slog.Logger,
	w http.ResponseWriter,
	status int,
	body string,
) {
	w.Header().Set(
		"Content-Type",
		"text/html; charset=utf-8",
	)
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		log.Error(
			"failed to write redirect response",
			"error",
			err,
		)
	}
}
