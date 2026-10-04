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

	"github.com/devflex/traffoflex/apps/traffic-service/internal/antirepeat"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/availability"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/clickaudit"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/clicklog"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/config"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/eventqueue"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/healthcheck"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/healthstate"
	apphttp "github.com/devflex/traffoflex/apps/traffic-service/internal/http"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/mongodb"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/requestctx"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/trafficevents"
	"github.com/devflex/traffoflex/packages/go-shared/clientip"
	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/logger"
	"github.com/devflex/traffoflex/packages/go-shared/models"
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

	mongoClient, err := mongodb.Open(
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

	store := cache.NewPersistentStore(
		cache.NewMongoLoader(mongoClient.Database(cfg.MongoDatabase)),
		cache.NewSnapshotFile(cfg.RoutingSnapshotPath),
	)
	initializeRouting(
		ctx,
		store,
		log,
	)
	go refreshCampaignCache(
		ctx,
		store,
		log,
		cfg.CampaignCacheRefreshInterval,
	)
	bloom, err := antirepeat.NewManager(antirepeat.Config{
		ExpectedKeysPerBucket: cfg.BloomExpectedKeys,
		FalsePositiveRate:     cfg.BloomFalsePositiveRate,
		MaxBytes:              int64(cfg.BloomMaxMemoryMB) * 1024 * 1024,
		BucketCount:           cfg.BloomBucketCount,
		SnapshotPath:          cfg.BloomSnapshotPath,
		SnapshotInterval:      cfg.BloomSnapshotInterval,
	})
	if err != nil {
		log.Error("invalid anti-repeat config", "error", err)
		os.Exit(1)
	}
	bloom.Configure(store.SnapshotCampaigns())
	if err := bloom.Load(); err != nil && !errors.Is(err, antirepeat.ErrNoSnapshot) {
		log.Warn("anti-repeat snapshot unavailable; history backfill required", "error", err)
	}
	bloomDone := bloom.Start(ctx, store, antirepeat.NewBackfill(cfg.ClickHouseHTTPURL), log)

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
	healthService := healthcheck.NewServiceWithThresholds(
		store,
		healthSink,
		healthClient,
		healthstate.NewMongoRepository(mongoClient.Database(cfg.MongoDatabase)),
		healthcheck.Thresholds{
			Failures: cfg.HealthcheckFailures,
			Recovery: cfg.HealthcheckRecovery,
		},
	)
	healthcheck.NewWorker(
		log,
		healthService,
		cfg.HealthcheckInterval,
		cfg.HealthcheckParallelism,
	).Start(ctx)
	clickSink := clicklog.NewKafkaSink(
		eventProducer,
		cfg.ClickEventsTopic,
	)
	clickQueue, err := eventqueue.OpenClickWAL(
		eventqueue.ClickWALConfig{
			Dir:          cfg.ClickWALPath,
			MaxBytes:     cfg.ClickWALMaxBytes,
			SegmentBytes: cfg.ClickWALSegmentBytes,
			BatchSize:    cfg.EventBatchSize,
			RetryDelay:   cfg.ClickWALRetryDelay,
		},
		log,
		clickSink.WriteBatch,
	)
	if err != nil {
		log.Error("failed to open click WAL", "error", err)
		os.Exit(1)
	}
	clickAudit := clickaudit.New(
		cfg.ClickHouseHTTPURL,
		log,
	)
	clickAudit.Start(ctx, clickQueue)
	trafficbackSink := trafficevents.NewKafkaTrafficbackSink(
		eventProducer,
		cfg.TrafficbackTopic,
	)
	trafficbackQueue := eventqueue.NewBatched[models.TrafficbackEvent](
		"trafficback",
		cfg.EventQueueSize,
		cfg.EventBatchSize,
		5*time.Millisecond,
		log,
		trafficbackSink.WriteBatch,
	)
	clickHouseAvailability := availability.NewClickHouseCapCheckerWithDoer(
		cfg.ClickHouseHTTPURL,
		nil,
		cfg.DestinationCapCacheTTL,
	)
	availabilitySnapshot := availability.NewSnapshot(
		store,
		clickHouseAvailability,
		log,
		cfg.DestinationCapCacheTTL,
	)
	availabilitySnapshot.Start(ctx)

	r := apphttp.NewRouterWithOptions(
		log,
		apphttp.Options{
			ReadyChecker:     store,
			Cache:            store,
			HealthClient:     healthClient,
			HealthService:    healthService,
			EventProducer:    eventProducer,
			ClickQueue:       clickQueue,
			ClickAudit:       clickAudit,
			TrafficbackQueue: trafficbackQueue,
			Availability: availability.NewEvaluatorWithSources(
				log,
				availabilitySnapshot,
				nil,
				availabilitySnapshot,
			),
			AntiRepeat:           bloom,
			AvailabilitySnapshot: availabilitySnapshot,
			Builder: requestctx.NewBuilder(clientip.NewResolver(
				prefixes,
				cfg.TrustedIPHeaders,
			)),
			ClickLogger: clickQueue,
			Trafficback: eventqueue.NewTrafficbackSink(trafficbackQueue),
			Health:      healthSink,
		},
	)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
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
	stop()
	<-shutdownDone
	drainCtx, cancelDrain := context.WithTimeout(
		context.Background(),
		cfg.EventDrainTimeout,
	)
	defer cancelDrain()
	clickDrain := make(chan error, 1)
	trafficbackDrain := make(chan error, 1)
	go func() { clickDrain <- clickQueue.Close(drainCtx) }()
	go func() { trafficbackDrain <- trafficbackQueue.Close(drainCtx) }()
	if err := <-clickDrain; err != nil {
		log.Warn("click WAL shutdown incomplete", "error", err, "stats", clickQueue.Stats())
	}
	if err := <-trafficbackDrain; err != nil {
		log.Warn("trafficback event queue drain incomplete", "error", err, "stats", trafficbackQueue.Stats())
	}
	<-clickQueue.Done()
	<-trafficbackQueue.Done()
	<-bloomDone
	if err := bloom.Save(); err != nil {
		log.Error("failed to save anti-repeat snapshot on shutdown", "error", err)
	}
}

func initializeRouting(
	ctx context.Context,
	store *cache.Store,
	log *slog.Logger,
) {
	restoreErr := store.Restore()
	if restoreErr != nil && !errors.Is(
		restoreErr,
		cache.ErrNoRoutingSnapshot,
	) {
		log.Warn(
			"routing snapshot unavailable",
			"error",
			restoreErr,
		)
	}
	loadCtx, cancelLoad := context.WithTimeout(
		ctx,
		5*time.Second,
	)
	loadErr := store.Reload(loadCtx)
	cancelLoad()
	if loadErr == nil {
		return
	}
	if restoreErr == nil {
		log.Warn(
			"serving restored routing snapshot while MongoDB is unavailable",
			"error",
			loadErr,
		)
		return
	}
	log.Warn(
		"no usable routing configuration yet; waiting for MongoDB",
		"mongo_error",
		loadErr,
		"snapshot_error",
		restoreErr,
	)
}

func refreshCampaignCache(
	ctx context.Context,
	store *cache.Store,
	log *slog.Logger,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			loadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := store.Reload(loadCtx)
			cancel()
			if err != nil {
				log.Warn("campaign cache refresh failed; retaining last good routing", "error", err)
			}
		}
	}
}
