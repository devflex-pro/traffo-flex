package destinations

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
	repo := NewMemoryRepository()
	service := NewService(repo)
	if err := service.SeedDemo(); err != nil {
		log.Error(
			"failed to seed demo destination",
			"error",
			err,
		)
	}
	return NewHandlerWithService(
		log,
		service,
	)
}

func NewHandlerWithService(
	log *slog.Logger,
	service *Service,
) *Handler {
	return &Handler{log: log, service: service}
}

func (h *Handler) List(
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
	destinations, err := h.service.List(r.Context())
	if err != nil {
		h.respondError(
			w,
			http.StatusInternalServerError,
			"failed to list destinations",
			err,
		)
		return
	}
	items, total := httpx.PaginateSlice(
		destinations,
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
	var req DestinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			"invalid JSON body",
			err,
		)
		return
	}

	destination, err := h.service.Create(
		r.Context(),
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
		"destination created",
		"destination_id",
		destination.ID,
	)
	h.respondJSON(
		w,
		http.StatusCreated,
		destination,
	)
}

func (h *Handler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	destination, err := h.service.Get(
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
	h.respondJSON(
		w,
		http.StatusOK,
		destination,
	)
}

func (h *Handler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req DestinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			"invalid JSON body",
			err,
		)
		return
	}

	destination, err := h.service.Update(
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
		destination,
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

func (h *Handler) Healthcheck(
	w http.ResponseWriter,
	r *http.Request,
) {
	destination, err := h.service.Get(
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
	h.respondJSON(w, http.StatusAccepted, map[string]string{
		"id":     destination.ID,
		"status": "scheduled",
	})
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
			"destination not found",
			err,
		)
	default:
		h.respondError(
			w,
			http.StatusInternalServerError,
			"destination operation failed",
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
			"failed to write destination response",
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
			"destination request failed",
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
			"failed to write destination error response",
			"error",
			writeErr,
		)
	}
}
