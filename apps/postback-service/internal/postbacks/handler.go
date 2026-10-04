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
	secrets     SecretStore
}

type ownerSecretStore interface {
	Credentials(context.Context, string) ([]normalize.Credential, error)
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
	return NewHandlerWithSecrets(
		log,
		service,
		outboundService,
		postbackLog,
		nil,
	)
}

func NewHandlerWithSecrets(
	log *slog.Logger,
	service *conversions.Service,
	outboundService *outbound.Service,
	postbackLog postbacklogs.Logger,
	secrets SecretStore,
) *Handler {
	return &Handler{log: log, conversions: service, outbound: outboundService, postbackLog: postbackLog, secrets: secrets}
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
	template, ok := h.loadSecrets(
		w,
		r,
		postbackID,
		network,
	)
	if !ok {
		return
	}
	conversion, err := normalize.FromGET(
		r,
		template,
	)
	if err != nil {
		h.respondNormalizeError(
			w,
			r.Context(),
			postbackID,
			ownerFromTemplate(template),
			network,
			nil,
			err,
		)
		return
	}
	event, created, err := h.conversions.Process(
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
	if created {
		h.logPostback(r, postbackID, "accepted", "", conversion)
		h.enqueueOutbound(r, event)
	} else {
		h.logPostback(r, postbackID, "duplicate", "", conversion)
	}

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
	template, ok := h.loadSecrets(
		w,
		r,
		postbackID,
		"api",
	)
	if !ok {
		return
	}
	conversion, err := normalize.FromPOST(
		r,
		template,
	)
	if err != nil {
		h.respondNormalizeError(
			w,
			r.Context(),
			postbackID,
			ownerFromTemplate(template),
			"api",
			nil,
			err,
		)
		return
	}
	event, created, err := h.conversions.Process(
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
	if created {
		h.logPostback(r, postbackID, "accepted", "", conversion)
		h.enqueueOutbound(r, event)
	} else {
		h.logPostback(r, postbackID, "duplicate", "", conversion)
	}

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

func (h *Handler) loadSecrets(
	w http.ResponseWriter,
	r *http.Request,
	postbackID string,
	networkID string,
) (normalize.Template, bool) {
	if h.secrets == nil {
		h.respondNormalizeError(
			w,
			r.Context(),
			postbackID,
			"",
			networkID,
			nil,
			normalize.ErrUnauthorized,
		)
		return normalize.Template{}, false
	}
	if ownerStore, ok := h.secrets.(ownerSecretStore); ok {
		credentials, err := ownerStore.Credentials(r.Context(), networkID)
		if err != nil {
			h.respondSecretLookupError(w, networkID, err)
			return normalize.Template{}, false
		}
		if len(credentials) == 0 {
			h.respondNormalizeError(
				w,
				r.Context(),
				postbackID,
				"",
				networkID,
				nil,
				normalize.ErrUnauthorized,
			)
			return normalize.Template{}, false
		}
		return normalize.Template{NetworkID: networkID, Credentials: credentials}, true
	}
	secrets, err := h.secrets.Secrets(
		r.Context(),
		networkID,
	)
	if err != nil {
		h.log.Warn(
			"postback secret lookup failed",
			"network",
			networkID,
			"error",
			err,
		)
		if writeErr := httpx.Error(
			w,
			http.StatusServiceUnavailable,
			"postback authentication unavailable",
		); writeErr != nil {
			h.log.Error("failed to write postback error", "error", writeErr)
		}
		return normalize.Template{}, false
	}
	if len(secrets) == 0 {
		h.respondNormalizeError(
			w,
			r.Context(),
			postbackID,
			"",
			networkID,
			nil,
			normalize.ErrUnauthorized,
		)
		return normalize.Template{}, false
	}
	return normalize.Template{NetworkID: networkID, Secrets: secrets}, true
}

func (h *Handler) respondSecretLookupError(w http.ResponseWriter, networkID string, err error) {
	h.log.Warn("postback secret lookup failed", "network", networkID, "error", err)
	if writeErr := httpx.Error(w, http.StatusServiceUnavailable, "postback authentication unavailable"); writeErr != nil {
		h.log.Error("failed to write postback error", "error", writeErr)
	}
}

func (h *Handler) respondNormalizeError(
	w http.ResponseWriter,
	ctx context.Context,
	postbackID string,
	ownerID string,
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
		ownerID,
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

func ownerFromTemplate(template normalize.Template) string {
	if len(template.Credentials) == 0 {
		return ""
	}
	ownerID := template.Credentials[0].OwnerID
	if ownerID == "" {
		return ""
	}
	for _, credential := range template.Credentials[1:] {
		if credential.OwnerID != ownerID {
			return ""
		}
	}
	return ownerID
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
	h.logPostbackEvent(
		ctx,
		postbackID,
		conversion.OwnerID,
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
		conversion.OwnerID,
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
	ownerID string,
	networkID string,
	clickID string,
	transactionID string,
	status string,
	errorMessage string,
	raw map[string]string,
) {
	event := models.PostbackLogEvent{
		PostbackID:    postbackID,
		OwnerID:       ownerID,
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
