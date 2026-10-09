-- Clicks -> hourly -> daily (UTC). Both views update on each inserted block.
-- Existing volumes require the controlled migration script; do not apply this
-- alongside an active minute/hour chain.
CREATE TABLE IF NOT EXISTS traffoflex.click_stats_1h
(
    created_at DateTime,
    owner_id String,
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
    source_campaign_name String,
    clicks UInt64,
    cost Float64
)
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (owner_id, created_at, campaign_id, stream_id, destination_id, source_id, geo_country, geo_region, city, device_type, os, browser, isp, zone_id, publisher_id, site_id, creative_id, carrier, connection_type, source_campaign_id, source_campaign_name);

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.click_events_to_stats_1h
TO traffoflex.click_stats_1h
AS
SELECT
    toStartOfHour(created_at) AS created_at,
    owner_id,
    campaign_id,
    stream_id,
    destination_id,
    source_id,
    geo_country,
    geo_region,
    city,
    device_type,
    os,
    browser,
    isp,
    if(notEmpty(JSONExtractString(query, 'zone_id')), JSONExtractString(query, 'zone_id'), sub1) AS zone_id,
    if(notEmpty(JSONExtractString(query, 'publisher_id')), JSONExtractString(query, 'publisher_id'), sub2) AS publisher_id,
    if(notEmpty(JSONExtractString(query, 'site_id')), JSONExtractString(query, 'site_id'), sub3) AS site_id,
    if(notEmpty(JSONExtractString(query, 'creative_id')), JSONExtractString(query, 'creative_id'), sub4) AS creative_id,
    JSONExtractString(query, 'carrier') AS carrier,
    JSONExtractString(query, 'source_connection_type') AS connection_type,
    JSONExtractString(query, 'source_campaign_id') AS source_campaign_id,
    JSONExtractString(query, 'source_campaign_name') AS source_campaign_name,
    count() AS clicks,
    sum(cost) AS cost
FROM traffoflex.click_events
GROUP BY created_at, owner_id, campaign_id, stream_id, destination_id, source_id, geo_country, geo_region, city, device_type, os, browser, isp, zone_id, publisher_id, site_id, creative_id, carrier, connection_type, source_campaign_id, source_campaign_name;

CREATE TABLE IF NOT EXISTS traffoflex.click_stats_1d AS traffoflex.click_stats_1h
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (owner_id, created_at, campaign_id, stream_id, destination_id, source_id, geo_country, geo_region, city, device_type, os, browser, isp, zone_id, publisher_id, site_id, creative_id, carrier, connection_type, source_campaign_id, source_campaign_name);

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.click_stats_1h_to_stats_1d
TO traffoflex.click_stats_1d
AS
SELECT
    toStartOfDay(created_at, 'UTC') AS created_at,
    owner_id, campaign_id, stream_id, destination_id, source_id, geo_country, geo_region, city, device_type, os, browser, isp, zone_id, publisher_id, site_id, creative_id, carrier, connection_type, source_campaign_id, source_campaign_name,
    sum(clicks) AS clicks,
    sum(cost) AS cost
FROM traffoflex.click_stats_1h
GROUP BY created_at, owner_id, campaign_id, stream_id, destination_id, source_id, geo_country, geo_region, city, device_type, os, browser, isp, zone_id, publisher_id, site_id, creative_id, carrier, connection_type, source_campaign_id, source_campaign_name;
