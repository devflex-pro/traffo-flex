package httpx

import "net/http"

type ReadyChecker interface {
	Ready(*http.Request) error
}

type ReadyFunc func(*http.Request) error

func (fn ReadyFunc) Ready(r *http.Request) error {
	return fn(r)
}

func HealthHandler() http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if err := Text(
			w,
			http.StatusOK,
			"ok",
		); err != nil {
			return
		}
	}
}

func ReadyHandler(checker ReadyChecker) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if checker != nil {
			if err := checker.Ready(r); err != nil {
				if err := Error(
					w,
					http.StatusServiceUnavailable,
					"not ready",
				); err != nil {
					return
				}
				return
			}
		}
		if err := Text(
			w,
			http.StatusOK,
			"ready",
		); err != nil {
			return
		}
	}
}
