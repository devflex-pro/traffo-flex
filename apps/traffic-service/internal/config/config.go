package config

import (
	"errors"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	sharedconfig "github.com/devflex/traffoflex/packages/go-shared/config"
)

type Config struct {
	Addr                         string
	MongoURI                     string
	MongoDatabase                string
	ClickHouseHTTPURL            string
	EventBrokers                 []string
	ClickEventsTopic             string
	TrafficbackTopic             string
	HealthEventsTopic            string
	EventBatchSize               int
	EventBatchTimeout            time.Duration
	EventWriteTimeout            time.Duration
	EventMaxAttempts             int
	EventRetryBackoff            time.Duration
	EventFailurePolicy           string
	EventQueueSize               int
	EventDrainTimeout            time.Duration
	ClickWALPath                 string
	ClickWALMaxBytes             int64
	ClickWALSegmentBytes         int64
	ClickWALRetryDelay           time.Duration
	HealthcheckTimeout           time.Duration
	HealthcheckInterval          time.Duration
	HealthcheckFailures          int
	HealthcheckRecovery          int
	HealthcheckParallelism       int
	DestinationCapCacheTTL       time.Duration
	BloomExpectedKeys            int
	BloomFalsePositiveRate       float64
	BloomMaxMemoryMB             int
	BloomBucketCount             int
	BloomSnapshotPath            string
	BloomSnapshotInterval        time.Duration
	RoutingSnapshotPath          string
	CampaignCacheRefreshInterval time.Duration
	TrustedProxyCIDRs            []string
	TrustedIPHeaders             []string
}

func Load() Config {
	return Config{
		Addr: sharedconfig.Env(
			"TRAFFIC_SERVICE_ADDR",
			":8080",
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
		ClickEventsTopic: sharedconfig.Env(
			"CLICK_EVENTS_TOPIC",
			"traffoflex.click_events",
		),
		TrafficbackTopic: sharedconfig.Env(
			"TRAFFICBACK_EVENTS_TOPIC",
			"traffoflex.trafficback_events",
		),
		HealthEventsTopic: sharedconfig.Env(
			"DESTINATION_HEALTH_EVENTS_TOPIC",
			"traffoflex.destination_health_events",
		),
		EventBatchSize: sharedconfig.EnvInt(
			"EVENT_BATCH_SIZE",
			100,
		),
		EventBatchTimeout: time.Duration(sharedconfig.EnvInt(
			"EVENT_BATCH_TIMEOUT_MS",
			10,
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
		EventFailurePolicy: sharedconfig.Env(
			"EVENT_WRITE_FAILURE_POLICY",
			"fail_open",
		),
		EventQueueSize: sharedconfig.EnvInt(
			"EVENT_QUEUE_SIZE",
			10000,
		),
		EventDrainTimeout: time.Duration(sharedconfig.EnvInt(
			"EVENT_DRAIN_TIMEOUT_MS",
			8000,
		)) * time.Millisecond,
		ClickWALPath:         sharedconfig.Env("CLICK_WAL_PATH", "/data/click-wal"),
		ClickWALMaxBytes:     int64(sharedconfig.EnvInt("CLICK_WAL_MAX_MB", 2048)) * 1024 * 1024,
		ClickWALSegmentBytes: int64(sharedconfig.EnvInt("CLICK_WAL_SEGMENT_MB", 16)) * 1024 * 1024,
		ClickWALRetryDelay:   time.Duration(sharedconfig.EnvInt("CLICK_WAL_RETRY_MS", 1000)) * time.Millisecond,
		HealthcheckTimeout: time.Duration(sharedconfig.EnvInt(
			"DESTINATION_HEALTHCHECK_TIMEOUT_MS",
			5000,
		)) * time.Millisecond,
		HealthcheckInterval: time.Duration(sharedconfig.EnvInt(
			"DESTINATION_HEALTHCHECK_INTERVAL_MS",
			60000,
		)) * time.Millisecond,
		HealthcheckFailures:    sharedconfig.EnvInt("DESTINATION_HEALTHCHECK_FAILURES", 3),
		HealthcheckRecovery:    sharedconfig.EnvInt("DESTINATION_HEALTHCHECK_RECOVERY", 2),
		HealthcheckParallelism: sharedconfig.EnvInt("DESTINATION_HEALTHCHECK_PARALLELISM", 4),
		DestinationCapCacheTTL: time.Duration(sharedconfig.EnvInt(
			"DESTINATION_CAP_CACHE_TTL_MS",
			10000,
		)) * time.Millisecond,
		BloomExpectedKeys:            sharedconfig.EnvInt("BLOOM_EXPECTED_KEYS_PER_BUCKET", 50000),
		BloomFalsePositiveRate:       envFloat("BLOOM_FALSE_POSITIVE_RATE", 0.001),
		BloomMaxMemoryMB:             sharedconfig.EnvInt("BLOOM_MAX_MEMORY_MB", 512),
		BloomBucketCount:             sharedconfig.EnvInt("BLOOM_BUCKET_COUNT", 24),
		BloomSnapshotPath:            sharedconfig.Env("BLOOM_SNAPSHOT_PATH", "/data/bloom/snapshot.gob"),
		BloomSnapshotInterval:        time.Duration(sharedconfig.EnvInt("BLOOM_SNAPSHOT_INTERVAL_MS", 30000)) * time.Millisecond,
		RoutingSnapshotPath:          sharedconfig.Env("ROUTING_SNAPSHOT_PATH", "/data/cache/campaigns.json"),
		CampaignCacheRefreshInterval: time.Duration(sharedconfig.EnvInt("CAMPAIGN_CACHE_REFRESH_INTERVAL_MS", 30000)) * time.Millisecond,
		TrustedProxyCIDRs: splitCSV(sharedconfig.Env(
			"TRUSTED_PROXY_CIDRS",
			"",
		)),
		TrustedIPHeaders: splitCSV(sharedconfig.Env(
			"TRUSTED_IP_HEADERS",
			"X-Forwarded-For,X-Real-IP",
		)),
	}
}

func envFloat(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(sharedconfig.Env(key, strconv.FormatFloat(fallback, 'f', -1, 64)), 64)
	if err != nil {
		return 0
	}
	return value
}

func (c Config) Validate() error {
	return errors.Join(
		sharedconfig.RequireNonEmpty(
			"TRAFFIC_SERVICE_ADDR",
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
		validateHealthcheck(c),
		validateDestinationCaps(c),
		validateCIDRs(c.TrustedProxyCIDRs),
		sharedconfig.RequireNonEmpty("ROUTING_SNAPSHOT_PATH", c.RoutingSnapshotPath),
		validateCampaignCache(c.CampaignCacheRefreshInterval),
	)
}

func validateCampaignCache(interval time.Duration) error {
	if interval <= 0 {
		return errors.New("CAMPAIGN_CACHE_REFRESH_INTERVAL_MS must be positive")
	}
	return nil
}

func (c Config) Ready(r *http.Request) error {
	return c.Validate()
}

func (c Config) TrustedProxyPrefixes() (
	[]netip.Prefix,
	error,
) {
	prefixes := make(
		[]netip.Prefix,
		0,
		len(c.TrustedProxyCIDRs),
	)
	for _, cidr := range c.TrustedProxyCIDRs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, err
		}
		prefixes = append(
			prefixes,
			prefix,
		)
	}
	return prefixes, nil
}

func validateCIDRs(cidrs []string) error {
	for _, cidr := range cidrs {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			return errors.New("TRUSTED_PROXY_CIDRS contains invalid CIDR")
		}
	}
	return nil
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
	if c.EventFailurePolicy != "fail_open" && c.EventFailurePolicy != "fail_closed" {
		return errors.New("EVENT_WRITE_FAILURE_POLICY must be fail_open or fail_closed")
	}
	if c.EventQueueSize <= 0 {
		return errors.New("EVENT_QUEUE_SIZE must be positive")
	}
	if c.EventDrainTimeout <= 0 {
		return errors.New("EVENT_DRAIN_TIMEOUT_MS must be positive")
	}
	if c.ClickWALPath == "" || c.ClickWALMaxBytes <= 0 || c.ClickWALSegmentBytes <= 0 || c.ClickWALSegmentBytes > c.ClickWALMaxBytes || c.ClickWALRetryDelay <= 0 {
		return errors.New("invalid click WAL configuration")
	}
	return errors.Join(
		sharedconfig.RequireNonEmpty(
			"CLICK_EVENTS_TOPIC",
			c.ClickEventsTopic,
		),
		sharedconfig.RequireNonEmpty(
			"TRAFFICBACK_EVENTS_TOPIC",
			c.TrafficbackTopic,
		),
		sharedconfig.RequireNonEmpty(
			"DESTINATION_HEALTH_EVENTS_TOPIC",
			c.HealthEventsTopic,
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

func validateHealthcheck(c Config) error {
	if c.HealthcheckTimeout <= 0 {
		return errors.New("DESTINATION_HEALTHCHECK_TIMEOUT_MS must be positive")
	}
	if c.HealthcheckInterval <= 0 {
		return errors.New("DESTINATION_HEALTHCHECK_INTERVAL_MS must be positive")
	}
	if c.HealthcheckFailures <= 0 || c.HealthcheckRecovery <= 0 || c.HealthcheckParallelism <= 0 || c.HealthcheckParallelism > 32 {
		return errors.New("destination healthcheck thresholds or parallelism are invalid")
	}
	return nil
}

func validateDestinationCaps(c Config) error {
	if c.DestinationCapCacheTTL <= 0 {
		return errors.New("DESTINATION_CAP_CACHE_TTL_MS must be positive")
	}
	return nil
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
