package internalapi

import (
	"log/slog"
	"net/http"

	"github.com/devflex/traffoflex/apps/postback-service/internal/outbound"
	"github.com/devflex/traffoflex/packages/go-shared/httpx"
)

type Handler struct {
	log      *slog.Logger
	outbound *outbound.Service
}

func NewHandler(
	log *slog.Logger,
	outboundService *outbound.Service,
) *Handler {
	if outboundService == nil {
		outboundService = outbound.NewService(
			nil,
			nil,
		)
	}
	return &Handler{log: log, outbound: outboundService}
}

func (h *Handler) RetryOutbound(
	w http.ResponseWriter,
	r *http.Request,
) {
	retried, err := h.outbound.Retry(r.Context())
	if err != nil {
		h.respondError(
			w,
			http.StatusInternalServerError,
			"outbound retry failed",
			err,
		)
		return
	}
	if err := httpx.JSON(
		w,
		http.StatusAccepted,
		map[string]any{"status": "accepted", "retried": retried},
	); err != nil {
		h.log.Error(
			"failed to write outbound retry response",
			"error",
			err,
		)
	}
}

func (h *Handler) TestPostback(
	w http.ResponseWriter,
	r *http.Request,
) {
	if err := httpx.JSON(
		w,
		http.StatusAccepted,
		map[string]string{"status": "accepted"},
	); err != nil {
		h.log.Error(
			"failed to write postback test response",
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
		"internal request failed",
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
			"failed to write internal error response",
			"error",
			writeErr,
		)
	}
}
