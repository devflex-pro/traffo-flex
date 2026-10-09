-- v0.1.6 schema fixture, only for isolated migration tests.
CREATE TABLE IF NOT EXISTS traffoflex.click_attribution_lookup
(
    created_at DateTime,
    owner_id String,
    click_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    source_id String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (owner_id, click_id)
SETTINGS index_granularity = 1024;

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.click_attribution_lookup_mv
TO traffoflex.click_attribution_lookup
AS
SELECT created_at, owner_id, click_id, campaign_id, stream_id, destination_id, source_id
FROM traffoflex.click_events;

-- Additive rollups. Existing click_events require a separate, bounded backfill.

CREATE TABLE IF NOT EXISTS traffoflex.click_stats_1m
(
    created_at DateTime,
    owner_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    source_id String,
    clicks UInt64,
    cost Float64
)
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (owner_id, created_at, campaign_id, stream_id, destination_id, source_id);

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.click_events_to_stats_1m
TO traffoflex.click_stats_1m
AS
SELECT
    toStartOfMinute(created_at) AS created_at,
    owner_id,
    campaign_id,
    stream_id,
    destination_id,
    source_id,
    count() AS clicks,
    sum(cost) AS cost
FROM traffoflex.click_events
GROUP BY owner_id, created_at, campaign_id, stream_id, destination_id, source_id;

CREATE TABLE IF NOT EXISTS traffoflex.click_stats_1h
(
    created_at DateTime,
    owner_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    source_id String,
    clicks UInt64,
    cost Float64
)
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (owner_id, created_at, campaign_id, stream_id, destination_id, source_id);

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.click_stats_1m_to_stats_1h
TO traffoflex.click_stats_1h
AS
SELECT
    toStartOfHour(created_at) AS created_at,
    owner_id,
    campaign_id,
    stream_id,
    destination_id,
    source_id,
    sum(clicks) AS clicks,
    sum(cost) AS cost
FROM traffoflex.click_stats_1m
GROUP BY owner_id, created_at, campaign_id, stream_id, destination_id, source_id;
