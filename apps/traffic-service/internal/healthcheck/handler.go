package healthcheck

import (
	"log/slog"
	"net/http"

	"github.com/devflex/traffoflex/packages/go-shared/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	log     *slog.Logger
	service *Service
}

func NewHandler(
	log *slog.Logger,
	service *Service,
) *Handler {
	return &Handler{log: log, service: service}
}

func (h *Handler) Trigger(
	w http.ResponseWriter,
	r *http.Request,
) {
	result, err := h.service.Trigger(
		r.Context(),
		chi.URLParam(
			r,
			"id",
		),
	)
	if err != nil {
		if IsNotFound(err) {
			h.respondError(
				w,
				http.StatusNotFound,
				"destination not found",
				err,
			)
			return
		}
		h.respondError(
			w,
			http.StatusInternalServerError,
			"healthcheck trigger failed",
			err,
		)
		return
	}
	if err := httpx.JSON(
		w,
		http.StatusAccepted,
		result,
	); err != nil {
		h.log.Error(
			"failed to write healthcheck response",
			"error",
			err,
		)
	}
}

func (h *Handler) respondError(
	w http.ResponseWriter,
	status int,
	message string,
	err error,
) {
	h.log.Warn(
		"healthcheck request failed",
		"status",
		status,
		"error",
		err,
	)
	if writeErr := httpx.Error(
		w,
		status,
		message,
	); writeErr != nil {
		h.log.Error(
			"failed to write healthcheck error response",
			"error",
			writeErr,
		)
	}
}
