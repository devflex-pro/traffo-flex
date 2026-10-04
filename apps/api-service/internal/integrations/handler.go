package integrations

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/devflex/traffoflex/apps/api-service/internal/destinations"
	"github.com/devflex/traffoflex/packages/go-shared/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	log          *slog.Logger
	client       Client
	destinations destinations.Repository
}

func NewHandler(
	log *slog.Logger,
	client Client,
	destinationRepo ...destinations.Repository,
) *Handler {
	if client == nil {
		client = NoopClient{}
	}
	h := &Handler{log: log, client: client}
	if len(destinationRepo) > 0 {
		h.destinations = destinationRepo[0]
	}
	return h
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
	if h.destinations != nil {
		if _, err := h.destinations.Get(
			r.Context(),
			destinationID,
		); err != nil {
			status := http.StatusInternalServerError
			message := "failed to check destination access"
			if errors.Is(err, destinations.ErrNotFound) {
				status = http.StatusNotFound
				message = "destination not found"
			}
			h.respondError(
				w,
				status,
				message,
				err,
			)
			return
		}
	}
	result, err := h.client.TriggerDestinationHealthcheck(
		r.Context(),
		destinationID,
	)
	if err != nil {
		h.respondError(
			w,
			http.StatusBadGateway,
			"failed to trigger destination healthcheck",
			err,
		)
		return
	}
	if err := httpx.JSON(w, http.StatusAccepted, result); err != nil {
		h.log.Error("failed to write healthcheck result", "error", err)
	}
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
