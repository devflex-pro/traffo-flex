package postbacks

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/devflex/traffoflex/apps/postback-service/internal/conversions"
	"github.com/devflex/traffoflex/apps/postback-service/internal/normalize"
	"github.com/devflex/traffoflex/apps/postback-service/internal/outbound"
	"github.com/devflex/traffoflex/apps/postback-service/internal/postbacklogs"
	"github.com/devflex/traffoflex/packages/go-shared/httpx"
	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	log         *slog.Logger
	conversions *conversions.Service
	outbound    *outbound.Service
	postbackLog postbacklogs.Logger
}

func NewHandler(log *slog.Logger) *Handler {
	return NewHandlerWithDeps(
		log,
		conversions.NewService(conversions.NewMemoryRepository()),
		outbound.NewService(
			nil,
			nil,
		),
		postbacklogs.NewMemoryLogger(),
	)
}

func NewHandlerWithService(
	log *slog.Logger,
	service *conversions.Service,
) *Handler {
	return NewHandlerWithDeps(
		log,
		service,
		outbound.NewService(
			nil,
			nil,
		),
		postbacklogs.NewMemoryLogger(),
	)
}

func NewHandlerWithDeps(
	log *slog.Logger,
	service *conversions.Service,
	outboundService *outbound.Service,
	postbackLog postbacklogs.Logger,
) *Handler {
	return &Handler{log: log, conversions: service, outbound: outboundService, postbackLog: postbackLog}
}

func (h *Handler) ReceiveGET(
	w http.ResponseWriter,
	r *http.Request,
) {
	network := chi.URLParam(
		r,
		"network",
	)
	postbackID := ids.New("pb")
	conversion, err := normalize.FromGET(
		r,
		normalize.Template{NetworkID: network},
	)
	if err != nil {
		h.respondNormalizeError(
			w,
			r.Context(),
			postbackID,
			network,
			nil,
			err,
		)
		return
	}
	event, err := h.conversions.Process(
		r.Context(),
		conversion,
	)
	if err != nil {
		h.respondConversionError(
			w,
			r.Context(),
			postbackID,
			conversion,
			err,
		)
		return
	}
	h.logPostback(
		r,
		postbackID,
		"accepted",
		"",
		conversion,
	)
	h.enqueueOutbound(
		r,
		event,
	)

	h.log.Info(
		"postback received",
		"postback_id",
		postbackID,
		"conversion_id",
		event.ConversionID,
		"network",
		network,
		"click_id",
		conversion.ClickID,
		"transaction_id",
		conversion.TransactionID,
		"status",
		conversion.Status,
	)

	if err := httpx.JSON(
		w,
		http.StatusOK,
		map[string]string{"status": "accepted", "postback_id": postbackID, "conversion_id": event.ConversionID, "click_id": conversion.ClickID, "transaction_id": conversion.TransactionID},
	); err != nil {
		h.log.Error(
			"failed to write postback response",
			"error",
			err,
		)
	}
}

func (h *Handler) ReceivePOST(
	w http.ResponseWriter,
	r *http.Request,
) {
	postbackID := ids.New("pb")
	conversion, err := normalize.FromPOST(
		r,
		normalize.Template{NetworkID: "api"},
	)
	if err != nil {
		h.respondNormalizeError(
			w,
			r.Context(),
			postbackID,
			"api",
			nil,
			err,
		)
		return
	}
	event, err := h.conversions.Process(
		r.Context(),
		conversion,
	)
	if err != nil {
		h.respondConversionError(
			w,
			r.Context(),
			postbackID,
			conversion,
			err,
		)
		return
	}
	h.logPostback(
		r,
		postbackID,
		"accepted",
		"",
		conversion,
	)
	h.enqueueOutbound(
		r,
		event,
	)

	h.log.Info(
		"postback received",
		"postback_id",
		postbackID,
		"conversion_id",
		event.ConversionID,
		"network",
		conversion.NetworkID,
		"click_id",
		conversion.ClickID,
		"transaction_id",
		conversion.TransactionID,
		"status",
		conversion.Status,
	)

	if err := httpx.JSON(
		w,
		http.StatusOK,
		map[string]string{"status": "accepted", "postback_id": postbackID, "conversion_id": event.ConversionID, "click_id": conversion.ClickID, "transaction_id": conversion.TransactionID},
	); err != nil {
		h.log.Error(
			"failed to write postback response",
			"error",
			err,
		)
	}
}

func (h *Handler) respondNormalizeError(
	w http.ResponseWriter,
	ctx context.Context,
	postbackID string,
	networkID string,
	raw map[string]string,
	err error,
) {
	status := http.StatusBadRequest
	message := "invalid postback payload"
	if errors.Is(
		err,
		normalize.ErrUnauthorized,
	) {
		status = http.StatusUnauthorized
		message = "invalid postback secret"
	}
	h.logPostbackEvent(
		ctx,
		postbackID,
		networkID,
		"",
		"",
		"rejected",
		err.Error(),
		raw,
	)
	h.log.Warn(
		"postback rejected",
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
			"failed to write postback error response",
			"error",
			writeErr,
		)
	}
}

func (h *Handler) respondConversionError(
	w http.ResponseWriter,
	ctx context.Context,
	postbackID string,
	conversion normalize.Conversion,
	err error,
) {
	status := http.StatusInternalServerError
	message := "conversion processing failed"
	if errors.Is(
		err,
		conversions.ErrDuplicate,
	) {
		status = http.StatusConflict
		message = "duplicate conversion"
	}
	h.logPostbackEvent(
		ctx,
		postbackID,
		conversion.NetworkID,
		conversion.ClickID,
		conversion.TransactionID,
		"rejected",
		err.Error(),
		conversion.RawPayload,
	)
	h.log.Warn(
		"conversion rejected",
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
			"failed to write conversion error response",
			"error",
			writeErr,
		)
	}
}

func (h *Handler) logPostback(
	r *http.Request,
	postbackID string,
	status string,
	errorMessage string,
	conversion normalize.Conversion,
) {
	h.logPostbackEvent(
		r.Context(),
		postbackID,
		conversion.NetworkID,
		conversion.ClickID,
		conversion.TransactionID,
		status,
		errorMessage,
		conversion.RawPayload,
	)
}

func (h *Handler) logPostbackEvent(
	ctx context.Context,
	postbackID string,
	networkID string,
	clickID string,
	transactionID string,
	status string,
	errorMessage string,
	raw map[string]string,
) {
	event := models.PostbackLogEvent{
		PostbackID:    postbackID,
		NetworkID:     networkID,
		ClickID:       clickID,
		TransactionID: transactionID,
		Status:        status,
		Error:         errorMessage,
		RawPayload:    raw,
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.postbackLog.Log(
		ctx,
		event,
	); err != nil {
		h.log.Warn(
			"failed to log postback event",
			"error",
			err,
			"postback_id",
			postbackID,
		)
	}
}

func (h *Handler) enqueueOutbound(
	r *http.Request,
	conversion models.ConversionEvent,
) {
	if h.outbound == nil {
		return
	}
	if err := h.outbound.EnqueueForConversion(
		r.Context(),
		conversion,
	); err != nil {
		h.log.Warn(
			"failed to enqueue outbound postback",
			"error",
			err,
			"conversion_id",
			conversion.ConversionID,
		)
	}
}
