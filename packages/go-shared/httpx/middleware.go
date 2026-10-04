package httpx

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (
	int,
	error,
) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

var requestSeq uint64

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set(
			"X-Request-ID",
			requestID,
		)
		next.ServeHTTP(
			w,
			r,
		)
	})
}

func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(
				sw,
				r,
			)
			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			log.Info("http request",
				"request_id", w.Header().Get("X-Request-ID"),
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", sw.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("http panic recovered",
						"request_id", w.Header().Get("X-Request-ID"),
						"method", r.Method,
						"path", r.URL.Path,
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					if err := Error(
						w,
						http.StatusInternalServerError,
						"internal server error",
					); err != nil {
						log.Error(
							"failed to write panic response",
							"error",
							err,
						)
					}
				}
			}()
			next.ServeHTTP(
				w,
				r,
			)
		})
	}
}

func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if allowedOrigin != "" {
				w.Header().Set(
					"Access-Control-Allow-Origin",
					allowedOrigin,
				)
				w.Header().Set(
					"Vary",
					"Origin",
				)
				w.Header().Set(
					"Access-Control-Allow-Headers",
					"Content-Type, Authorization, X-Request-ID, X-TraffoFlex-CSRF, X-TraffoFlex-Act-As",
				)
				w.Header().Set(
					"Access-Control-Allow-Credentials",
					"true",
				)
				w.Header().Set(
					"Access-Control-Allow-Methods",
					"GET, POST, PUT, DELETE, OPTIONS",
				)
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(
				w,
				r,
			)
		})
	}
}

func newRequestID() string {
	seq := atomic.AddUint64(
		&requestSeq,
		1,
	)
	return strconv.FormatInt(
		time.Now().UnixNano(),
		36,
	) + "-" + strconv.FormatUint(
		seq,
		36,
	)
}
