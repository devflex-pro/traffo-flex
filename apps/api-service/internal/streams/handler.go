package streams

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/devflex/traffoflex/packages/go-shared/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	log     *slog.Logger
	service *Service
}

func NewHandler(log *slog.Logger) *Handler {
	return NewHandlerWithService(
		log,
		NewService(NewMemoryRepository()),
	)
}

func NewHandlerWithService(
	log *slog.Logger,
	service *Service,
) *Handler {
	return &Handler{log: log, service: service}
}

func (h *Handler) ListByCampaign(
	w http.ResponseWriter,
	r *http.Request,
) {
	pagination, err := httpx.ParsePagination(r)
	if err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			err.Error(),
			err,
		)
		return
	}
	streams, err := h.service.ListByCampaign(
		r.Context(),
		chi.URLParam(
			r,
			"id",
		),
	)
	if err != nil {
		h.respondServiceError(
			w,
			err,
		)
		return
	}
	items, total := httpx.PaginateSlice(
		streams,
		pagination,
	)
	h.respondJSON(
		w,
		http.StatusOK,
		httpx.NewListResponse(
			items,
			pagination,
			total,
		),
	)
}

func (h *Handler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req StreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			"invalid JSON body",
			err,
		)
		return
	}

	stream, err := h.service.Create(
		r.Context(),
		chi.URLParam(
			r,
			"id",
		),
		req,
	)
	if err != nil {
		h.respondServiceError(
			w,
			err,
		)
		return
	}

	h.log.Info(
		"stream created",
		"stream_id",
		stream.ID,
		"campaign_id",
		stream.CampaignID,
	)
	h.respondJSON(
		w,
		http.StatusCreated,
		stream,
	)
}

func (h *Handler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req StreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			"invalid JSON body",
			err,
		)
		return
	}

	stream, err := h.service.Update(
		r.Context(),
		chi.URLParam(
			r,
			"id",
		),
		req,
	)
	if err != nil {
		h.respondServiceError(
			w,
			err,
		)
		return
	}
	h.respondJSON(
		w,
		http.StatusOK,
		stream,
	)
}

func (h *Handler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	if err := h.service.Delete(
		r.Context(),
		chi.URLParam(
			r,
			"id",
		),
	); err != nil {
		h.respondServiceError(
			w,
			err,
		)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) respondServiceError(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		ErrInvalidInput,
	):
		h.respondError(
			w,
			http.StatusBadRequest,
			err.Error(),
			err,
		)
	case errors.Is(
		err,
		ErrNotFound,
	):
		h.respondError(
			w,
			http.StatusNotFound,
			"stream not found",
			err,
		)
	default:
		h.respondError(
			w,
			http.StatusInternalServerError,
			"stream operation failed",
			err,
		)
	}
}

func (h *Handler) respondJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	if err := httpx.JSON(
		w,
		status,
		data,
	); err != nil {
		h.log.Error(
			"failed to write stream response",
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
	if err != nil {
		h.log.Warn(
			"stream request failed",
			"status",
			status,
			"error",
			err,
		)
	}
	if writeErr := httpx.Error(
		w,
		status,
		message,
	); writeErr != nil {
		h.log.Error(
			"failed to write stream error response",
			"error",
			writeErr,
		)
	}
}
