package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/availability"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/clicklog"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/config"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/healthcheck"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/healthstate"
	apphttp "github.com/devflex/traffoflex/apps/traffic-service/internal/http"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/mongodb"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/requestctx"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/trafficevents"
	"github.com/devflex/traffoflex/packages/go-shared/clientip"
	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	cfg := config.Load()
	log := logger.New("traffic-service")
	if err := cfg.Validate(); err != nil {
		log.Error(
			"invalid traffic-service config",
			slog.String(
				"error",
				err.Error(),
			),
		)
		os.Exit(1)
	}

	prefixes, err := cfg.TrustedProxyPrefixes()
	if err != nil {
		log.Error(
			"invalid trusted proxy config",
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

	store := cache.NewStore(cache.NewMongoLoader(mongoClient.Database(cfg.MongoDatabase)))
	if err := store.Reload(context.Background()); err != nil {
		log.Error(
			"failed to load campaign cache",
			slog.String(
				"error",
				err.Error(),
			),
		)
		os.Exit(1)
	}

	eventProducer, err := eventstream.NewProducer(eventstream.Config{
		Brokers:      cfg.EventBrokers,
		ClientID:     "traffic-service",
		BatchSize:    cfg.EventBatchSize,
		BatchTimeout: cfg.EventBatchTimeout,
		WriteTimeout: cfg.EventWriteTimeout,
		MaxAttempts:  cfg.EventMaxAttempts,
		RetryBackoff: cfg.EventRetryBackoff,
		Logger:       log,
	})
	if err != nil {
		log.Error(
			"failed to create event producer",
			slog.String(
				"error",
				err.Error(),
			),
		)
		os.Exit(1)
	}
	defer func() {
		if err := eventProducer.Close(); err != nil {
			log.Error(
				"failed to close event producer",
				slog.String(
					"error",
					err.Error(),
				),
			)
		}
	}()

	healthClient := &http.Client{
		Timeout: cfg.HealthcheckTimeout,
	}
	healthEvents := trafficevents.NewKafkaDestinationHealthSink(
		eventProducer,
		cfg.HealthEventsTopic,
	)
	var healthSink trafficevents.DestinationHealthSink = healthEvents
	if cfg.EventFailurePolicy == "fail_open" {
		healthSink = trafficevents.NewBestEffortDestinationHealthSink(
			log,
			healthEvents,
		)
	}
	healthService := healthcheck.NewServiceWithDependencies(
		store,
		healthSink,
		healthClient,
		healthstate.NewMongoRepository(mongoClient.Database(cfg.MongoDatabase)),
	)
	healthcheck.NewWorker(
		log,
		healthService,
		cfg.HealthcheckInterval,
	).Start(ctx)

	r := apphttp.NewRouterWithOptions(
		log,
		apphttp.Options{
			ReadyChecker:  cfg,
			Cache:         store,
			HealthClient:  healthClient,
			EventProducer: eventProducer,
			Availability: availability.NewEvaluator(
				log,
				availability.NewClickHouseCapCheckerWithDoer(
					cfg.ClickHouseHTTPURL,
					nil,
					cfg.DestinationCapCacheTTL,
				),
			),
			Builder: requestctx.NewBuilder(clientip.NewResolver(
				prefixes,
				cfg.TrustedIPHeaders,
			)),
			ClickLogger: clicklog.NewSinkLogger(
				clicklog.NewKafkaSink(
					eventProducer,
					cfg.ClickEventsTopic,
				),
			),
			Trafficback: trafficevents.NewKafkaTrafficbackSink(
				eventProducer,
				cfg.TrafficbackTopic,
			),
			Health: healthSink,
		},
	)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error(
				"failed to shutdown traffic-service",
				slog.String(
					"error",
					err.Error(),
				),
			)
		}
	}()

	log.Info(
		"starting traffic-service",
		"addr",
		cfg.Addr,
	)
	if err := server.ListenAndServe(); err != nil && !errors.Is(
		err,
		http.ErrServerClosed,
	) {
		log.Error(
			"traffic-service stopped",
			slog.String(
				"error",
				err.Error(),
			),
		)
	}
}
