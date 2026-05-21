CREATE TABLE IF NOT EXISTS traffoflex.click_events_queue
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
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0:9092,redpanda-1:9092,redpanda-2:9092',
    kafka_topic_list = 'traffoflex.click_events',
    kafka_group_name = 'traffoflex_click_events_clickhouse',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1,
    kafka_handle_error_mode = 'stream';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.click_events_mv
TO traffoflex.click_events
AS
SELECT
    created_at,
    click_id,
    campaign_id,
    stream_id,
    destination_id,
    source_id,
    source_click_id,
    ip_hash,
    ip_prefix,
    user_agent,
    ua_hash,
    geo_country,
    geo_region,
    city,
    asn,
    isp,
    device_type,
    os,
    browser,
    referrer,
    sub1,
    sub2,
    sub3,
    sub4,
    sub5,
    sub6,
    sub7,
    sub8,
    sub9,
    sub10,
    utm_source,
    utm_medium,
    utm_campaign,
    utm_content,
    utm_term,
    cost,
    currency,
    is_bot,
    bot_score,
    raw_query,
    query
FROM traffoflex.click_events_queue
WHERE _error = '';

CREATE TABLE IF NOT EXISTS traffoflex.conversion_events_queue
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
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0:9092,redpanda-1:9092,redpanda-2:9092',
    kafka_topic_list = 'traffoflex.conversion_events',
    kafka_group_name = 'traffoflex_conversion_events_clickhouse',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1,
    kafka_handle_error_mode = 'stream';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.conversion_events_mv
TO traffoflex.conversion_events
AS
SELECT
    created_at,
    updated_at,
    conversion_id,
    click_id,
    transaction_id,
    campaign_id,
    stream_id,
    destination_id,
    source_id,
    offer_id,
    event_type,
    status,
    payout,
    currency,
    network_id,
    raw_payload
FROM traffoflex.conversion_events_queue
WHERE _error = '';

CREATE TABLE IF NOT EXISTS traffoflex.postback_log_events_queue
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
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0:9092,redpanda-1:9092,redpanda-2:9092',
    kafka_topic_list = 'traffoflex.postback_log_events',
    kafka_group_name = 'traffoflex_postback_log_events_clickhouse',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1,
    kafka_handle_error_mode = 'stream';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.postback_log_events_mv
TO traffoflex.postback_log_events
AS
SELECT
    created_at,
    postback_id,
    network_id,
    click_id,
    transaction_id,
    status,
    error,
    raw_payload
FROM traffoflex.postback_log_events_queue
WHERE _error = '';

CREATE TABLE IF NOT EXISTS traffoflex.trafficback_events_queue
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
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0:9092,redpanda-1:9092,redpanda-2:9092',
    kafka_topic_list = 'traffoflex.trafficback_events',
    kafka_group_name = 'traffoflex_trafficback_events_clickhouse',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1,
    kafka_handle_error_mode = 'stream';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.trafficback_events_mv
TO traffoflex.trafficback_events
AS
SELECT
    created_at,
    click_id,
    campaign_id,
    stream_id,
    destination_id,
    reason,
    depth,
    visited_destinations
FROM traffoflex.trafficback_events_queue
WHERE _error = '';

CREATE TABLE IF NOT EXISTS traffoflex.destination_health_events_queue
(
    created_at DateTime,
    destination_id String,
    previous String,
    current String,
    error String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0:9092,redpanda-1:9092,redpanda-2:9092',
    kafka_topic_list = 'traffoflex.destination_health_events',
    kafka_group_name = 'traffoflex_destination_health_events_clickhouse',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1,
    kafka_handle_error_mode = 'stream';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.destination_health_events_mv
TO traffoflex.destination_health_events
AS
SELECT
    created_at,
    destination_id,
    previous,
    current,
    error
FROM traffoflex.destination_health_events_queue
WHERE _error = '';

CREATE TABLE IF NOT EXISTS traffoflex.kafka_ingestion_errors
(
    observed_at DateTime,
    topic String,
    error String,
    raw_message String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(observed_at)
ORDER BY (topic, observed_at);

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.click_events_errors_mv
TO traffoflex.kafka_ingestion_errors
AS
SELECT
    now() AS observed_at,
    'traffoflex.click_events' AS topic,
    _error AS error,
    _raw_message AS raw_message
FROM traffoflex.click_events_queue
WHERE _error != '';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.conversion_events_errors_mv
TO traffoflex.kafka_ingestion_errors
AS
SELECT
    now() AS observed_at,
    'traffoflex.conversion_events' AS topic,
    _error AS error,
    _raw_message AS raw_message
FROM traffoflex.conversion_events_queue
WHERE _error != '';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.postback_log_events_errors_mv
TO traffoflex.kafka_ingestion_errors
AS
SELECT
    now() AS observed_at,
    'traffoflex.postback_log_events' AS topic,
    _error AS error,
    _raw_message AS raw_message
FROM traffoflex.postback_log_events_queue
WHERE _error != '';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.trafficback_events_errors_mv
TO traffoflex.kafka_ingestion_errors
AS
SELECT
    now() AS observed_at,
    'traffoflex.trafficback_events' AS topic,
    _error AS error,
    _raw_message AS raw_message
FROM traffoflex.trafficback_events_queue
WHERE _error != '';

CREATE MATERIALIZED VIEW IF NOT EXISTS traffoflex.destination_health_events_errors_mv
TO traffoflex.kafka_ingestion_errors
AS
SELECT
    now() AS observed_at,
    'traffoflex.destination_health_events' AS topic,
    _error AS error,
    _raw_message AS raw_message
FROM traffoflex.destination_health_events_queue
WHERE _error != '';
