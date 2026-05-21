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

	"github.com/devflex/traffoflex/apps/postback-service/internal/config"
	"github.com/devflex/traffoflex/apps/postback-service/internal/conversions"
	apphttp "github.com/devflex/traffoflex/apps/postback-service/internal/http"
	"github.com/devflex/traffoflex/apps/postback-service/internal/mongodb"
	"github.com/devflex/traffoflex/apps/postback-service/internal/postbacklogs"
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
	log := logger.New("postback-service")
	if err := cfg.Validate(); err != nil {
		log.Error(
			"invalid postback-service config",
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
	eventProducer, err := eventstream.NewProducer(eventstream.Config{
		Brokers:      cfg.EventBrokers,
		ClientID:     "postback-service",
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

	var conversionSink conversions.EventSink = conversions.NewKafkaSink(
		eventProducer,
		cfg.ConversionTopic,
	)
	var postbackEventLogger postbacklogs.Logger = postbacklogs.NewKafkaLogger(
		eventProducer,
		cfg.PostbackLogTopic,
	)
	if cfg.EventFailurePolicy == "fail_open" {
		conversionSink = conversions.NewBestEffortEventSink(
			log,
			conversionSink,
		)
		postbackEventLogger = postbacklogs.NewBestEffortLogger(
			log,
			postbackEventLogger,
		)
	}
	var clickLookup conversions.ClickLookup = conversions.NewClickHouseClickLookup(
		cfg.ClickHouseHTTPURL,
		cfg.ClickLookupTimeout,
	)
	if cfg.ClickLookupPolicy == "fail_open" {
		clickLookup = conversions.NewBestEffortClickLookup(
			log,
			clickLookup,
		)
	}

	r := apphttp.NewRouterWithOptions(
		log,
		apphttp.Options{
			ReadyChecker:         cfg,
			ConversionRepository: conversions.NewMongoRepository(db),
			ConversionSink:       conversionSink,
			ClickLookup:          clickLookup,
			EventProducer:        eventProducer,
			PostbackLogger: postbacklogs.NewMultiLogger(
				postbacklogs.NewMongoLogger(db),
				postbackEventLogger,
			),
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
				"failed to shutdown postback-service",
				slog.String(
					"error",
					err.Error(),
				),
			)
		}
	}()

	log.Info(
		"starting postback-service",
		"addr",
		cfg.Addr,
	)
	if err := server.ListenAndServe(); err != nil && !errors.Is(
		err,
		http.ErrServerClosed,
	) {
		log.Error(
			"postback-service stopped",
			slog.String(
				"error",
				err.Error(),
			),
		)
	}
}
