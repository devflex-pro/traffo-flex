CREATE TABLE IF NOT EXISTS traffoflex.click_attribution_lookup
(
    created_at DateTime,
    owner_id String,
    click_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    source_id String,
    geo_country String,
    geo_region String,
    city String,
    device_type String,
    os String,
    browser String,
    isp String,
    zone_id String,
    publisher_id String,
    site_id String,
    creative_id String,
    carrier String,
    connection_type String,
    source_campaign_id String,
    source_campaign_name String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (owner_id, click_id)
SETTINGS index_granularity = 1024;

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.click_attribution_lookup_mv
TO traffoflex.click_attribution_lookup
AS
SELECT created_at, owner_id, click_id, campaign_id, stream_id, destination_id, source_id,
    geo_country, geo_region, city, device_type, os, browser, isp, if(notEmpty(JSONExtractString(query, 'zone_id')), JSONExtractString(query, 'zone_id'), sub1) AS zone_id, if(notEmpty(JSONExtractString(query, 'publisher_id')), JSONExtractString(query, 'publisher_id'), sub2) AS publisher_id, if(notEmpty(JSONExtractString(query, 'site_id')), JSONExtractString(query, 'site_id'), sub3) AS site_id, if(notEmpty(JSONExtractString(query, 'creative_id')), JSONExtractString(query, 'creative_id'), sub4) AS creative_id, JSONExtractString(query, 'carrier') AS carrier, JSONExtractString(query, 'source_connection_type') AS connection_type, JSONExtractString(query, 'source_campaign_id') AS source_campaign_id, JSONExtractString(query, 'source_campaign_name') AS source_campaign_name
FROM traffoflex.click_events;

CREATE TABLE IF NOT EXISTS traffoflex.attributed_conversion_events
(
    created_at DateTime,
    attributed_at DateTime,
    owner_id String,
    conversion_id String,
    click_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    source_id String,
    payout Float64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (owner_id, created_at, conversion_id);

CREATE TABLE IF NOT EXISTS traffoflex.attributed_conversion_events_queue
(
    created_at DateTime,
    attributed_at DateTime,
    owner_id String,
    conversion_id String,
    click_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    source_id String,
    payout Float64
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0:9092,redpanda-1:9092,redpanda-2:9092',
    kafka_topic_list = 'traffoflex.attributed_conversion_events',
    kafka_group_name = 'traffoflex_attributed_conversions_clickhouse',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1,
    kafka_handle_error_mode = 'stream';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.attributed_conversion_events_mv
TO traffoflex.attributed_conversion_events
AS
SELECT created_at, attributed_at, owner_id, conversion_id, click_id,
       campaign_id, stream_id, destination_id, source_id, payout
FROM traffoflex.attributed_conversion_events_queue;

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.attributed_conversion_events_errors_mv
TO traffoflex.kafka_ingestion_errors
AS
SELECT
    now() AS observed_at,
    'traffoflex.attributed_conversion_events' AS topic,
    _error AS error,
    _raw_message AS raw_message
FROM traffoflex.attributed_conversion_events_queue
WHERE _error != '';
