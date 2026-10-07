package http

import (
	"log/slog"
	"net/http"

	"github.com/devflex/traffoflex/apps/postback-service/internal/conversions"
	"github.com/devflex/traffoflex/apps/postback-service/internal/internalapi"
	"github.com/devflex/traffoflex/apps/postback-service/internal/outbound"
	"github.com/devflex/traffoflex/apps/postback-service/internal/postbacklogs"
	"github.com/devflex/traffoflex/apps/postback-service/internal/postbacks"
	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/httpx"
	"github.com/go-chi/chi/v5"
)

func NewRouter(log *slog.Logger) http.Handler {
	return NewRouterWithReadyChecker(
		log,
		nil,
	)
}

func NewRouterWithReadyChecker(
	log *slog.Logger,
	ready httpx.ReadyChecker,
) http.Handler {
	return NewRouterWithOptions(
		log,
		Options{ReadyChecker: ready},
	)
}

type Options struct {
	ReadyChecker         httpx.ReadyChecker
	ConversionRepository conversions.Repository
	PostbackLogger       postbacklogs.Logger
	EventProducer        *eventstream.Producer
	PostbackSecrets      postbacks.SecretStore
	OutboundWorker       *outbound.PersistentWorker
}

func NewRouterWithOptions(
	log *slog.Logger,
	opts Options,
) http.Handler {
	r := chi.NewRouter()
	h := postbacks.NewHandlerWithSecrets(
		log,
		conversions.NewService(conversions.NewMemoryRepository()),
		nil,
		postbacklogs.NewMemoryLogger(),
		opts.PostbackSecrets,
	)
	if opts.ConversionRepository != nil || opts.PostbackLogger != nil {
		repo := opts.ConversionRepository
		if repo == nil {
			repo = conversions.NewMemoryRepository()
		}
		postbackLogger := opts.PostbackLogger
		if postbackLogger == nil {
			postbackLogger = postbacklogs.NewMemoryLogger()
		}
		h = postbacks.NewHandlerWithSecrets(
			log,
			conversions.NewService(repo),
			nil,
			postbackLogger,
			opts.PostbackSecrets,
		)
	}
	var retryer internalapi.Retryer
	if opts.OutboundWorker != nil {
		retryer = opts.OutboundWorker
	}
	internalHandler := internalapi.NewHandler(
		log,
		retryer,
	)

	r.Use(httpx.RequestID)
	r.Use(httpx.Recover(log))
	r.Use(httpx.Logging(log))
	r.Use(httpx.CORS(""))

	r.Get(
		"/healthz",
		httpx.HealthHandler(),
	)
	r.Get(
		"/readyz",
		httpx.ReadyHandler(opts.ReadyChecker),
	)
	if opts.EventProducer != nil {
		r.Get(
			"/internal/event-producer/stats",
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				if err := httpx.JSON(
					w,
					http.StatusOK,
					opts.EventProducer.Stats(),
				); err != nil {
					log.Error(
						"failed to write event producer stats response",
						"error",
						err,
					)
				}
			},
		)
	}

	r.Get(
		"/pb/{network}",
		h.ReceiveGET,
	)
	r.Post(
		"/api/postbacks",
		h.ReceivePOST,
	)
	r.Post(
		"/internal/outbound/retry",
		internalHandler.RetryOutbound,
	)
	r.Post(
		"/internal/postbacks/test",
		internalHandler.TestPostback,
	)

	return r
}
