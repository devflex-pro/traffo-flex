CREATE TABLE IF NOT EXISTS traffoflex.clicks
(
    event_date Date,
    event_time DateTime,
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
    asn UInt32,
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
    bot_score UInt8,
    raw_query String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (campaign_id, event_date, event_time, click_id);

CREATE TABLE IF NOT EXISTS traffoflex.conversions
(
    event_date Date,
    event_time DateTime,
    conversion_id String,
    click_id String,
    transaction_id String,
    campaign_id String,
    stream_id String,
    destination_id String,
    offer_id String,
    event_type String,
    status String,
    payout Float64,
    currency String,
    network_id String,
    raw_payload String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (campaign_id, event_date, event_time, click_id);

CREATE TABLE IF NOT EXISTS traffoflex.trafficback_events_legacy
(
    event_date Date,
    event_time DateTime,
    click_id String,
    campaign_id String,
    stream_id String,
    original_destination_id String,
    trafficback_level String,
    trafficback_reason String,
    trafficback_url String,
    trafficback_depth UInt8,
    trafficback_chain String,
    source_id String,
    source_click_id String,
    geo_country String,
    device_type String,
    cost Float64,
    currency String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (campaign_id, event_date, event_time, click_id);

CREATE TABLE IF NOT EXISTS traffoflex.postback_logs
(
    event_date Date,
    event_time DateTime,
    postback_id String,
    network_id String,
    click_id String,
    transaction_id String,
    status String,
    payout Float64,
    currency String,
    is_valid UInt8,
    is_duplicate UInt8,
    error_type String,
    raw_payload String,
    ip String,
    user_agent String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (network_id, event_date, event_time, click_id);

CREATE TABLE IF NOT EXISTS traffoflex.destination_health_events_legacy
(
    event_date Date,
    event_time DateTime,
    destination_id String,
    campaign_id String,
    stream_id String,
    url String,
    method String,
    status_code UInt16,
    response_time_ms UInt32,
    is_success UInt8,
    error_type String,
    error_message String,
    redirect_chain Array(String),
    final_url String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (destination_id, event_date, event_time);
