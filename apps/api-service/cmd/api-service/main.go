package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/devflex/traffoflex/apps/api-service/internal/affiliatenetworks"
	"github.com/devflex/traffoflex/apps/api-service/internal/auth"
	"github.com/devflex/traffoflex/apps/api-service/internal/campaigns"
	"github.com/devflex/traffoflex/apps/api-service/internal/config"
	"github.com/devflex/traffoflex/apps/api-service/internal/destinations"
	"github.com/devflex/traffoflex/apps/api-service/internal/healthhistory"
	apphttp "github.com/devflex/traffoflex/apps/api-service/internal/http"
	"github.com/devflex/traffoflex/apps/api-service/internal/integrations"
	"github.com/devflex/traffoflex/apps/api-service/internal/mongodb"
	"github.com/devflex/traffoflex/apps/api-service/internal/postbacklogs"
	"github.com/devflex/traffoflex/apps/api-service/internal/postbacktemplates"
	"github.com/devflex/traffoflex/apps/api-service/internal/reports"
	"github.com/devflex/traffoflex/apps/api-service/internal/streams"
	"github.com/devflex/traffoflex/apps/api-service/internal/trafficsources"
	"github.com/devflex/traffoflex/packages/go-shared/logger"
)

func main() {
	cfg := config.Load()
	log := logger.New("api-service")
	if err := cfg.Validate(); err != nil {
		log.Error(
			"invalid api-service config",
			slog.String(
				"error",
				err.Error(),
			),
		)
		os.Exit(1)
	}

	mongoClient, err := mongodb.Connect(
		context.Background(),
		cfg.MongoURI,
	)
	if err != nil {
		log.Error(
			"failed to connect mongo",
			slog.String(
				"error",
				err.Error(),
			),
		)
		os.Exit(1)
	}
	defer func() {
		if err := mongodb.Disconnect(
			context.Background(),
			mongoClient,
		); err != nil {
			log.Error(
				"failed to disconnect mongo",
				slog.String(
					"error",
					err.Error(),
				),
			)
		}
	}()

	db := mongoClient.Database(cfg.MongoDatabase)
	r := apphttp.NewRouterWithOptions(
		log,
		apphttp.Options{
			ReadyChecker:        cfg,
			AdminFrontendOrigin: cfg.AdminFrontendOrigin,
			TrackerPublicURL:    cfg.TrackerPublicURL,
			PostbackPublicURL:   cfg.PostbackPublicURL,
			IntegrationClient: integrations.NewHTTPClient(
				cfg.TrafficServiceURL,
				cfg.PostbackServiceURL,
			),
			ReportRepository:           reports.NewClickHouseRepository(cfg.ClickHouseHTTPURL),
			CampaignRepository:         campaigns.NewMongoRepository(db),
			DestinationRepository:      destinations.NewMongoRepository(db),
			StreamRepository:           streams.NewMongoRepository(db),
			SourceRepository:           trafficsources.NewMongoRepository(db),
			NetworkRepository:          affiliatenetworks.NewMongoRepository(db),
			PostbackTemplateRepository: postbacktemplates.NewMongoRepository(db),
			PostbackLogRepository:      postbacklogs.NewMongoRepository(db),
			HealthHistoryRepository:    healthhistory.NewMongoRepository(db),
			AuthService: auth.NewServiceWithDelivery(
				auth.NewMongoRepository(db),
				authDelivery(
					log,
					cfg,
				),
				auth.Config{
					AdminEmail:    cfg.AuthAdminEmail,
					JWTSecret:     cfg.AuthJWTSecret,
					OTPTTL:        time.Duration(cfg.AuthOTPTTLSeconds) * time.Second,
					OTPRateLimit:  time.Duration(cfg.AuthOTPRateLimit) * time.Minute,
					SessionTTL:    time.Duration(cfg.AuthSessionTTL) * time.Hour,
					DevReturnOTP:  cfg.AuthDevReturnOTP,
					CookieSecure:  !cfg.LocalAuthEnv(),
					AllowedOrigin: cfg.AdminFrontendOrigin,
				},
			),
		},
	)

	log.Info(
		"starting api-service",
		"addr",
		cfg.Addr,
	)
	if err := http.ListenAndServe(
		cfg.Addr,
		r,
	); err != nil {
		log.Error(
			"api-service stopped",
			slog.String(
				"error",
				err.Error(),
			),
		)
	}
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
