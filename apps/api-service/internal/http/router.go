package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/devflex/traffoflex/apps/api-service/internal/affiliatenetworks"
	"github.com/devflex/traffoflex/apps/api-service/internal/auth"
	"github.com/devflex/traffoflex/apps/api-service/internal/campaigns"
	"github.com/devflex/traffoflex/apps/api-service/internal/config"
	"github.com/devflex/traffoflex/apps/api-service/internal/destinations"
	"github.com/devflex/traffoflex/apps/api-service/internal/healthhistory"
	"github.com/devflex/traffoflex/apps/api-service/internal/integrations"
	"github.com/devflex/traffoflex/apps/api-service/internal/postbacklogs"
	"github.com/devflex/traffoflex/apps/api-service/internal/postbacktemplates"
	"github.com/devflex/traffoflex/apps/api-service/internal/reports"
	"github.com/devflex/traffoflex/apps/api-service/internal/streams"
	"github.com/devflex/traffoflex/apps/api-service/internal/trafficsources"
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
	ReadyChecker               httpx.ReadyChecker
	AdminFrontendOrigin        string
	IntegrationClient          integrations.Client
	ReportRepository           reports.Repository
	CampaignRepository         campaigns.Repository
	DestinationRepository      destinations.Repository
	StreamRepository           streams.Repository
	SourceRepository           trafficsources.Repository
	NetworkRepository          affiliatenetworks.Repository
	PostbackTemplateRepository postbacktemplates.Repository
	PostbackLogRepository      postbacklogs.Repository
	HealthHistoryRepository    healthhistory.Repository
	AuthService                *auth.Service
}

func NewRouterWithConfig(
	log *slog.Logger,
	cfg config.Config,
) http.Handler {
	return NewRouterWithOptions(
		log,
		Options{
			ReadyChecker:        cfg,
			AdminFrontendOrigin: cfg.AdminFrontendOrigin,
			IntegrationClient: integrations.NewHTTPClient(
				cfg.TrafficServiceURL,
				cfg.PostbackServiceURL,
			),
			ReportRepository: reports.NewClickHouseRepository(cfg.ClickHouseHTTPURL),
			AuthService: auth.NewServiceWithDelivery(
				auth.NewMemoryRepository(),
				authDelivery(
					log,
					cfg,
				),
				auth.Config{
					AdminEmail:   cfg.AuthAdminEmail,
					JWTSecret:    cfg.AuthJWTSecret,
					OTPTTL:       time.Duration(cfg.AuthOTPTTLSeconds) * time.Second,
					OTPRateLimit: time.Duration(cfg.AuthOTPRateLimit) * time.Minute,
					SessionTTL:   time.Duration(cfg.AuthSessionTTL) * time.Hour,
					DevReturnOTP: cfg.AuthDevReturnOTP,
				},
			),
		},
	)
}

func authDelivery(
	log *slog.Logger,
	cfg config.Config,
) auth.OTPDelivery {
	if cfg.ResendConfigured() {
		return auth.NewResendDelivery(auth.ResendConfig{
			APIKey:       cfg.AuthResendAPIKey,
			APIURL:       cfg.AuthResendAPIURL,
			From:         cfg.AuthEmailFrom,
			MaxAttempts:  cfg.AuthResendMaxAttempts,
			RetryBackoff: time.Duration(cfg.AuthResendRetryBackoff) * time.Millisecond,
			Logger:       log,
		})
	}
	if cfg.LocalAuthEnv() {
		return auth.NewLogDelivery(log)
	}
	return nil
}

func NewRouterWithOptions(
	log *slog.Logger,
	opts Options,
) http.Handler {
	r := chi.NewRouter()
	campaignHandler := campaigns.NewHandler(log)
	if opts.CampaignRepository != nil {
		campaignHandler = campaigns.NewHandlerWithService(
			log,
			campaigns.NewService(opts.CampaignRepository),
		)
	}
	destinationHandler := destinations.NewHandler(log)
	if opts.DestinationRepository != nil {
		destinationHandler = destinations.NewHandlerWithService(
			log,
			destinations.NewService(opts.DestinationRepository),
		)
	}
	integrationHandler := integrations.NewHandler(
		log,
		opts.IntegrationClient,
	)
	healthHistoryHandler := healthhistory.NewHandler(
		log,
		opts.HealthHistoryRepository,
	)
	networkHandler := affiliatenetworks.NewHandler(log)
	if opts.NetworkRepository != nil {
		networkHandler = affiliatenetworks.NewHandlerWithService(
			log,
			affiliatenetworks.NewService(opts.NetworkRepository),
		)
	}
	postbackTemplateHandler := postbacktemplates.NewHandler(log)
	if opts.PostbackTemplateRepository != nil {
		postbackTemplateHandler = postbacktemplates.NewHandlerWithService(
			log,
			postbacktemplates.NewService(opts.PostbackTemplateRepository),
		)
	}
	postbackLogHandler := postbacklogs.NewHandler(
		log,
		opts.PostbackLogRepository,
	)
	reportHandler := reports.NewHandlerWithRepository(
		log,
		opts.ReportRepository,
	)
	authHandler := auth.NewHandler(
		log,
		opts.AuthService,
	)
	streamHandler := streams.NewHandler(log)
	if opts.StreamRepository != nil {
		streamHandler = streams.NewHandlerWithService(
			log,
			streams.NewService(opts.StreamRepository),
		)
	}
	sourceHandler := trafficsources.NewHandler(log)
	if opts.SourceRepository != nil {
		sourceHandler = trafficsources.NewHandlerWithService(
			log,
			trafficsources.NewService(opts.SourceRepository),
		)
	}

	r.Use(httpx.RequestID)
	r.Use(httpx.Recover(log))
	r.Use(httpx.Logging(log))
	r.Use(httpx.CORS(opts.AdminFrontendOrigin))

	r.Get(
		"/healthz",
		httpx.HealthHandler(),
	)
	r.Get(
		"/readyz",
		httpx.ReadyHandler(opts.ReadyChecker),
	)

	r.Route(
		"/api",
		func(r chi.Router) {
			if opts.AuthService != nil {
				r.Post(
					"/auth/request-otp",
					authHandler.RequestOTP,
				)
				r.Post(
					"/auth/verify-otp",
					authHandler.VerifyOTP,
				)
				r.Group(func(r chi.Router) {
					r.Use(authHandler.Middleware)
					registerProtectedRoutes(
						r,
						authHandler,
						campaignHandler,
						destinationHandler,
						integrationHandler,
						healthHistoryHandler,
						networkHandler,
						postbackTemplateHandler,
						postbackLogHandler,
						reportHandler,
						streamHandler,
						sourceHandler,
					)
				})
				return
			}

			registerProtectedRoutes(
				r,
				authHandler,
				campaignHandler,
				destinationHandler,
				integrationHandler,
				healthHistoryHandler,
				networkHandler,
				postbackTemplateHandler,
				postbackLogHandler,
				reportHandler,
				streamHandler,
				sourceHandler,
			)
		},
	)

	return r
}

func registerProtectedRoutes(
	r chi.Router,
	authHandler *auth.Handler,
	campaignHandler *campaigns.Handler,
	destinationHandler *destinations.Handler,
	integrationHandler *integrations.Handler,
	healthHistoryHandler *healthhistory.Handler,
	networkHandler *affiliatenetworks.Handler,
	postbackTemplateHandler *postbacktemplates.Handler,
	postbackLogHandler *postbacklogs.Handler,
	reportHandler *reports.Handler,
	streamHandler *streams.Handler,
	sourceHandler *trafficsources.Handler,
) {
	r.Get(
		"/me",
		authHandler.Me,
	)
	r.Get(
		"/users",
		authHandler.AdminOnly(http.HandlerFunc(authHandler.ListUsers)).ServeHTTP,
	)
	r.Post(
		"/users/{id}/approve",
		authHandler.AdminOnly(http.HandlerFunc(authHandler.ApproveUser)).ServeHTTP,
	)

	r.Get(
		"/campaigns",
		campaignHandler.List,
	)
	r.Post(
		"/campaigns",
		campaignHandler.Create,
	)
	r.Get(
		"/campaigns/{id}",
		campaignHandler.Get,
	)
	r.Put(
		"/campaigns/{id}",
		campaignHandler.Update,
	)
	r.Delete(
		"/campaigns/{id}",
		campaignHandler.Delete,
	)
	r.Get(
		"/campaigns/{id}/streams",
		streamHandler.ListByCampaign,
	)
	r.Post(
		"/campaigns/{id}/streams",
		streamHandler.Create,
	)

	r.Put(
		"/streams/{id}",
		streamHandler.Update,
	)
	r.Delete(
		"/streams/{id}",
		streamHandler.Delete,
	)

	r.Get(
		"/destinations",
		destinationHandler.List,
	)
	r.Get(
		"/destinations/health-history",
		healthHistoryHandler.List,
	)
	r.Post(
		"/destinations",
		destinationHandler.Create,
	)
	r.Get(
		"/destinations/{id}",
		destinationHandler.Get,
	)
	r.Put(
		"/destinations/{id}",
		destinationHandler.Update,
	)
	r.Delete(
		"/destinations/{id}",
		destinationHandler.Delete,
	)
	r.Post(
		"/destinations/{id}/healthcheck",
		integrationHandler.TriggerDestinationHealthcheck,
	)

	r.Get(
		"/traffic-sources",
		sourceHandler.List,
	)
	r.Post(
		"/traffic-sources",
		sourceHandler.Create,
	)
	r.Get(
		"/traffic-sources/{id}",
		sourceHandler.Get,
	)
	r.Put(
		"/traffic-sources/{id}",
		sourceHandler.Update,
	)
	r.Delete(
		"/traffic-sources/{id}",
		sourceHandler.Delete,
	)

	r.Get(
		"/affiliate-networks",
		networkHandler.List,
	)
	r.Post(
		"/affiliate-networks",
		networkHandler.Create,
	)
	r.Get(
		"/affiliate-networks/{id}",
		networkHandler.Get,
	)
	r.Put(
		"/affiliate-networks/{id}",
		networkHandler.Update,
	)
	r.Delete(
		"/affiliate-networks/{id}",
		networkHandler.Delete,
	)

	r.Get(
		"/postback-templates",
		postbackTemplateHandler.List,
	)
	r.Get(
		"/postback-logs",
		postbackLogHandler.List,
	)
	r.Post(
		"/postback-templates",
		postbackTemplateHandler.Create,
	)
	r.Get(
		"/postback-templates/{id}",
		postbackTemplateHandler.Get,
	)
	r.Put(
		"/postback-templates/{id}",
		postbackTemplateHandler.Update,
	)
	r.Delete(
		"/postback-templates/{id}",
		postbackTemplateHandler.Delete,
	)

	r.Post(
		"/postbacks/test",
		integrationHandler.TestPostback,
	)
	r.Post(
		"/internal/traffic/cache/reload",
		integrationHandler.ReloadTrafficCache,
	)

	r.Get(
		"/reports/overview",
		reportHandler.Overview,
	)
	r.Get(
		"/reports/daily",
		reportHandler.Daily,
	)
	r.Get(
		"/reports/campaigns",
		reportHandler.Campaigns,
	)
	r.Get(
		"/reports/streams",
		reportHandler.Streams,
	)
	r.Get(
		"/reports/destinations",
		reportHandler.Destinations,
	)
	r.Get(
		"/reports/sources",
		reportHandler.Sources,
	)
	r.Get(
		"/reports/trafficback",
		reportHandler.Trafficback,
	)
	r.Get(
		"/reports/health",
		reportHandler.Health,
	)
	r.Get(
		"/reports/ingestion-errors",
		reportHandler.IngestionErrors,
	)
}
