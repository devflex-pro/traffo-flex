package http

import (
	"log/slog"
	"net/http"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/availability"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/clicklog"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/config"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/healthcheck"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/redirects"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/requestctx"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/trafficevents"
	"github.com/devflex/traffoflex/packages/go-shared/clientip"
	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/httpx"
	"github.com/go-chi/chi/v5"
)

func NewRouter(log *slog.Logger) http.Handler {
	return NewRouterWithOptions(
		log,
		Options{},
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
	ReadyChecker  httpx.ReadyChecker
	Cache         *cache.Store
	Builder       *requestctx.Builder
	ClickLogger   clicklog.Logger
	Trafficback   trafficevents.TrafficbackSink
	Health        trafficevents.DestinationHealthSink
	HealthClient  *http.Client
	EventProducer *eventstream.Producer
	Availability  *availability.Evaluator
}

func NewRouterWithConfig(
	log *slog.Logger,
	cfg config.Config,
) http.Handler {
	prefixes, err := cfg.TrustedProxyPrefixes()
	if err != nil {
		log.Error(
			"invalid trusted proxy config",
			"error",
			err,
		)
	}
	return NewRouterWithOptions(log, Options{
		ReadyChecker: cfg,
		Builder: requestctx.NewBuilder(clientip.NewResolver(
			prefixes,
			cfg.TrustedIPHeaders,
		)),
	})
}

func NewRouterWithOptions(
	log *slog.Logger,
	opts Options,
) http.Handler {
	store := opts.Cache
	if store == nil {
		store = cache.NewDemoStore()
	}

	r := chi.NewRouter()
	healthcheckHandler := healthcheck.NewHandler(
		log,
		healthcheck.NewServiceWithClient(
			store,
			opts.Health,
			opts.HealthClient,
		),
	)
	redirectHandler := redirects.NewHandler(
		log,
		store,
		opts.Builder,
		opts.ClickLogger,
		opts.Trafficback,
		opts.Availability,
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
		"/c/{campaignSlug}",
		redirectHandler.Campaign,
	)
	r.Get(
		"/go/{campaignPublicId}",
		redirectHandler.Go,
	)
	r.Get(
		"/r/{publicToken}",
		redirectHandler.Token,
	)
	r.Get(
		"/tb/{campaignSlug}",
		redirectHandler.Trafficback,
	)
	r.Post(
		"/internal/destinations/{id}/healthcheck",
		healthcheckHandler.Trigger,
	)
	r.Post(
		"/internal/cache/reload",
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if err := store.Reload(r.Context()); err != nil {
				if writeErr := httpx.Error(
					w,
					http.StatusInternalServerError,
					"cache reload failed",
				); writeErr != nil {
					log.Error(
						"failed to write cache reload error response",
						"error",
						writeErr,
					)
				}
				return
			}
			if err := httpx.JSON(
				w,
				http.StatusAccepted,
				map[string]string{"status": "reloaded"},
			); err != nil {
				log.Error(
					"failed to write cache reload response",
					"error",
					err,
				)
			}
		},
	)

	return r
}
