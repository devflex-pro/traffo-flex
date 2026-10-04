package config

import (
	"errors"
	"net/http"
	"strings"
	"time"

	sharedconfig "github.com/devflex/traffoflex/packages/go-shared/config"
)

type Config struct {
	Addr              string
	MongoURI          string
	MongoDatabase     string
	ClickHouseHTTPURL string
	EventBrokers      []string
	ConversionTopic   string
	AttributionTopic  string
	PostbackLogTopic  string
	EventBatchSize    int
	EventBatchTimeout time.Duration
	EventWriteTimeout time.Duration
	EventMaxAttempts  int
	EventRetryBackoff time.Duration
}

func Load() Config {
	return Config{
		Addr: sharedconfig.Env(
			"POSTBACK_SERVICE_ADDR",
			":8081",
		),
		MongoURI: sharedconfig.Env(
			"MONGO_URI",
			"mongodb://mongo:27017",
		),
		MongoDatabase: sharedconfig.Env(
			"MONGO_DATABASE",
			"traffoflex",
		),
		ClickHouseHTTPURL: sharedconfig.Env(
			"CLICKHOUSE_HTTP_URL",
			"http://default:traffoflex@clickhouse:8123?database=traffoflex",
		),
		EventBrokers: splitCSV(sharedconfig.Env(
			"EVENT_BROKERS",
			"redpanda-0:9092,redpanda-1:9092,redpanda-2:9092",
		)),
		ConversionTopic: sharedconfig.Env(
			"CONVERSION_EVENTS_TOPIC",
			"traffoflex.conversion_events",
		),
		AttributionTopic: sharedconfig.Env(
			"ATTRIBUTED_CONVERSION_EVENTS_TOPIC",
			"traffoflex.attributed_conversion_events",
		),
		PostbackLogTopic: sharedconfig.Env(
			"POSTBACK_LOG_EVENTS_TOPIC",
			"traffoflex.postback_log_events",
		),
		EventBatchSize: sharedconfig.EnvInt(
			"EVENT_BATCH_SIZE",
			100,
		),
		EventBatchTimeout: time.Duration(sharedconfig.EnvInt(
			"EVENT_BATCH_TIMEOUT_MS",
			1000,
		)) * time.Millisecond,
		EventWriteTimeout: time.Duration(sharedconfig.EnvInt(
			"EVENT_WRITE_TIMEOUT_MS",
			3000,
		)) * time.Millisecond,
		EventMaxAttempts: sharedconfig.EnvInt(
			"EVENT_WRITE_MAX_ATTEMPTS",
			3,
		),
		EventRetryBackoff: time.Duration(sharedconfig.EnvInt(
			"EVENT_WRITE_RETRY_BACKOFF_MS",
			100,
		)) * time.Millisecond,
	}
}

func (c Config) Validate() error {
	return errors.Join(
		sharedconfig.RequireNonEmpty(
			"POSTBACK_SERVICE_ADDR",
			c.Addr,
		),
		sharedconfig.RequireURL(
			"MONGO_URI",
			c.MongoURI,
			"mongodb",
			"mongodb+srv",
		),
		sharedconfig.RequireNonEmpty(
			"MONGO_DATABASE",
			c.MongoDatabase,
		),
		sharedconfig.RequireURL(
			"CLICKHOUSE_HTTP_URL",
			c.ClickHouseHTTPURL,
			"http",
			"https",
		),
		validateEventStream(c),
	)
}

func (c Config) Ready(r *http.Request) error {
	return c.Validate()
}

func validateEventStream(c Config) error {
	if len(c.EventBrokers) == 0 {
		return errors.New("EVENT_BROKERS is required")
	}
	if c.EventBatchTimeout <= 0 {
		return errors.New("EVENT_BATCH_TIMEOUT_MS must be positive")
	}
	if c.EventWriteTimeout <= 0 {
		return errors.New("EVENT_WRITE_TIMEOUT_MS must be positive")
	}
	if c.EventRetryBackoff <= 0 {
		return errors.New("EVENT_WRITE_RETRY_BACKOFF_MS must be positive")
	}
	return errors.Join(
		sharedconfig.RequireNonEmpty(
			"CONVERSION_EVENTS_TOPIC",
			c.ConversionTopic,
		),
		sharedconfig.RequireNonEmpty(
			"ATTRIBUTED_CONVERSION_EVENTS_TOPIC",
			c.AttributionTopic,
		),
		sharedconfig.RequireNonEmpty(
			"POSTBACK_LOG_EVENTS_TOPIC",
			c.PostbackLogTopic,
		),
		sharedconfig.RequirePositive(
			"EVENT_BATCH_SIZE",
			c.EventBatchSize,
		),
		sharedconfig.RequirePositive(
			"EVENT_WRITE_MAX_ATTEMPTS",
			c.EventMaxAttempts,
		),
	)
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(
		value,
		",",
	)
	items := make(
		[]string,
		0,
		len(parts),
	)
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(
				items,
				item,
			)
		}
	}
	return items
}
