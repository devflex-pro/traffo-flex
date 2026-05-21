package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

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
	return &Handler{
		log:     log,
		service: service,
	}
}

type requestOTPBody struct {
	Email string `json:"email"`
}

type verifyOTPBody struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

func (h *Handler) RequestOTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req requestOTPBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			"invalid JSON body",
			err,
		)
		return
	}
	challenge, err := h.service.RequestOTP(
		r.Context(),
		req.Email,
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
		http.StatusAccepted,
		challenge,
	)
}

func (h *Handler) VerifyOTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req verifyOTPBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			"invalid JSON body",
			err,
		)
		return
	}
	session, err := h.service.VerifyOTP(
		r.Context(),
		req.Email,
		req.OTP,
	)
	if err != nil {
		if errors.Is(
			err,
			ErrPendingApproval,
		) {
			h.respondJSON(
				w,
				http.StatusForbidden,
				map[string]any{
					"error":  "user pending admin approval",
					"status": StatusPendingApproval,
					"user":   publicUser(session.User),
				},
			)
			return
		}
		h.respondServiceError(
			w,
			err,
		)
		return
	}
	h.respondJSON(
		w,
		http.StatusOK,
		map[string]any{
			"token": session.Token,
			"user":  publicUser(session.User),
		},
	)
}

func (h *Handler) Me(
	w http.ResponseWriter,
	r *http.Request,
) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		h.respondError(
			w,
			http.StatusUnauthorized,
			"authentication required",
			nil,
		)
		return
	}
	h.respondJSON(
		w,
		http.StatusOK,
		publicUser(user),
	)
}

func (h *Handler) ListUsers(
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
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		h.respondServiceError(
			w,
			err,
		)
		return
	}
	response := make(
		[]map[string]any,
		0,
		len(users),
	)
	for _, user := range users {
		response = append(
			response,
			publicUser(user),
		)
	}
	items, total := httpx.PaginateSlice(
		response,
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

func (h *Handler) ApproveUser(
	w http.ResponseWriter,
	r *http.Request,
) {
	user, err := h.service.ApproveUser(
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
		publicUser(user),
	)
}

func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(
			header,
			"Bearer ",
		) {
			h.respondError(
				w,
				http.StatusUnauthorized,
				"authentication required",
				nil,
			)
			return
		}
		user, err := h.service.Authenticate(
			r.Context(),
			strings.TrimSpace(strings.TrimPrefix(
				header,
				"Bearer ",
			)),
		)
		if err != nil {
			h.respondServiceError(
				w,
				err,
			)
			return
		}
		next.ServeHTTP(
			w,
			r.WithContext(ContextWithUser(
				r.Context(),
				user,
			)),
		)
	})
}

func (h *Handler) AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		user, ok := UserFromContext(r.Context())
		if !ok || user.Role != RoleAdmin {
			h.respondServiceError(
				w,
				ErrForbidden,
			)
			return
		}
		next.ServeHTTP(
			w,
			r,
		)
	})
}

func publicUser(user User) map[string]any {
	return map[string]any{
		"id":             user.ID,
		"email":          user.Email,
		"role":           user.Role,
		"status":         user.Status,
		"email_verified": user.EmailVerified,
		"approved":       user.Approved,
	}
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
		ErrInvalidOTP,
	):
		h.respondError(
			w,
			http.StatusUnauthorized,
			"invalid or expired otp",
			err,
		)
	case errors.Is(
		err,
		ErrInvalidToken,
	):
		h.respondError(
			w,
			http.StatusUnauthorized,
			"invalid auth token",
			err,
		)
	case errors.Is(
		err,
		ErrPendingApproval,
	):
		h.respondError(
			w,
			http.StatusForbidden,
			"user pending admin approval",
			err,
		)
	case errors.Is(
		err,
		ErrOTPActive,
	):
		h.respondError(
			w,
			http.StatusConflict,
			"otp is still active",
			err,
		)
	case errors.Is(
		err,
		ErrOTPRateLimited,
	):
		h.respondError(
			w,
			http.StatusTooManyRequests,
			"otp request rate limited",
			err,
		)
	case errors.Is(
		err,
		ErrForbidden,
	):
		h.respondError(
			w,
			http.StatusForbidden,
			"forbidden",
			err,
		)
	case errors.Is(
		err,
		ErrNotFound,
	):
		h.respondError(
			w,
			http.StatusNotFound,
			"user not found",
			err,
		)
	default:
		h.respondError(
			w,
			http.StatusInternalServerError,
			"auth operation failed",
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
			"failed to write auth response",
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
			"auth request failed",
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
			"failed to write auth error",
			"error",
			writeErr,
		)
	}
}
