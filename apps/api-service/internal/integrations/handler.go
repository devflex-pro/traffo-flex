package integrations

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/devflex/traffoflex/packages/go-shared/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	log    *slog.Logger
	client Client
}

func NewHandler(
	log *slog.Logger,
	client Client,
) *Handler {
	if client == nil {
		client = NoopClient{}
	}
	return &Handler{log: log, client: client}
}

func (h *Handler) ReloadTrafficCache(
	w http.ResponseWriter,
	r *http.Request,
) {
	if err := h.client.ReloadTrafficCache(r.Context()); err != nil {
		h.respondError(
			w,
			http.StatusBadGateway,
			"failed to reload traffic cache",
			err,
		)
		return
	}
	h.respondAccepted(
		w,
		"traffic cache reload triggered",
	)
}

func (h *Handler) TriggerDestinationHealthcheck(
	w http.ResponseWriter,
	r *http.Request,
) {
	destinationID := chi.URLParam(
		r,
		"id",
	)
	if err := h.client.TriggerDestinationHealthcheck(
		r.Context(),
		destinationID,
	); err != nil {
		h.respondError(
			w,
			http.StatusBadGateway,
			"failed to trigger destination healthcheck",
			err,
		)
		return
	}
	h.respondAccepted(
		w,
		"destination healthcheck triggered",
	)
}

func (h *Handler) TestPostback(
	w http.ResponseWriter,
	r *http.Request,
) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			"failed to read request body",
			err,
		)
		return
	}
	if err := h.client.TestPostback(
		r.Context(),
		payload,
	); err != nil {
		h.respondError(
			w,
			http.StatusBadGateway,
			"failed to test postback",
			err,
		)
		return
	}
	h.respondAccepted(
		w,
		"postback test triggered",
	)
}

func (h *Handler) respondAccepted(
	w http.ResponseWriter,
	message string,
) {
	if err := httpx.JSON(
		w,
		http.StatusAccepted,
		map[string]string{"status": "accepted", "message": message},
	); err != nil {
		h.log.Error(
			"failed to write integration response",
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
		"integration request failed",
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
			"failed to write integration error response",
			"error",
			writeErr,
		)
	}
}
