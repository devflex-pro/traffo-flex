CREATE TABLE IF NOT EXISTS traffoflex.click_events
(
    created_at DateTime,
    click_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    source_id String,
    source_click_id String,
    ip_hash String,
    ip_prefix String,
    user_agent String,
    ua_hash String,
    geo_country String,
    geo_region String,
    city String,
    asn String,
    isp String,
    device_type String,
    os String,
    browser String,
    referrer String,
    sub1 String,
    sub2 String,
    sub3 String,
    sub4 String,
    sub5 String,
    sub6 String,
    sub7 String,
    sub8 String,
    sub9 String,
    sub10 String,
    utm_source String,
    utm_medium String,
    utm_campaign String,
    utm_content String,
    utm_term String,
    cost Float64,
    currency String,
    is_bot UInt8,
    bot_score Float64,
    raw_query String,
    query String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (campaign_id, created_at, click_id);

CREATE TABLE IF NOT EXISTS traffoflex.conversion_events
(
    created_at DateTime,
    updated_at DateTime,
    conversion_id String,
    click_id String,
    transaction_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    source_id String,
    offer_id String,
    event_type String,
    status String,
    payout Float64,
    currency String,
    network_id String,
    raw_payload String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (campaign_id, created_at, click_id, transaction_id);

CREATE TABLE IF NOT EXISTS traffoflex.postback_log_events
(
    created_at DateTime,
    postback_id String,
    network_id String,
    click_id String,
    transaction_id String,
    status String,
    error String,
    raw_payload String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (network_id, created_at, postback_id);

CREATE TABLE IF NOT EXISTS traffoflex.trafficback_events
(
    created_at DateTime,
    click_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    reason String,
    depth UInt8,
    visited_destinations Array(String)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (campaign_id, created_at, click_id);

CREATE TABLE IF NOT EXISTS traffoflex.destination_health_events
(
    created_at DateTime,
    destination_id String,
    previous String,
    current String,
    error String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (destination_id, created_at);
