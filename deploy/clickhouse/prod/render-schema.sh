#!/bin/sh
set -eu

source_dir=${SCHEMA_SOURCE_DIR:-/tmp/traffoflex-schema}
target_dir=${SCHEMA_TARGET_DIR:-/docker-entrypoint-initdb.d}
mkdir -p "$target_dir"

cat > "$target_dir/001_create_database.sql" <<'SQL'
CREATE DATABASE IF NOT EXISTS traffoflex
ENGINE = Replicated('/clickhouse/databases/traffoflex', '{shard}', '{replica}');
SQL

for number in 002 003 004 005 006; do
    source_file=$(find "$source_dir" -maxdepth 1 -name "${number}_*.sql" -print)
    if [ -z "$source_file" ]; then
        echo "missing ClickHouse schema file ${number}" >&2
        exit 1
    fi
    target_file="$target_dir/$(basename "$source_file")"
    awk '
        /^CREATE TABLE IF NOT EXISTS traffoflex\./ || /^TO traffoflex\./ || /^FROM traffoflex\./ {
            if (match($0, /traffoflex\.[A-Za-z0-9_]+/)) {
                table = substr($0, RSTART + 11, RLENGTH - 11)
                if (table !~ /_queue$/) {
                    sub("traffoflex\\." table, "traffoflex." table "_local")
                }
            }
        }
        /^ENGINE = MergeTree$/ { $0 = "ENGINE = ReplicatedMergeTree" }
        /^ENGINE = SummingMergeTree$/ { $0 = "ENGINE = ReplicatedSummingMergeTree" }
        {
            gsub(/redpanda-0:9092,redpanda-1:9092,redpanda-2:9092/, "redpanda-0:9092")
            print
        }
    ' "$source_file" > "$target_file"
done

cat > "$target_dir/007_distributed_tables.sql" <<'SQL'
-- Public names are Distributed facades; materialized views write to _local tables.
CREATE TABLE IF NOT EXISTS traffoflex.clicks AS traffoflex.clicks_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'clicks_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.conversions AS traffoflex.conversions_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'conversions_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.trafficback_events_legacy AS traffoflex.trafficback_events_legacy_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'trafficback_events_legacy_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.postback_logs AS traffoflex.postback_logs_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'postback_logs_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.destination_health_events_legacy AS traffoflex.destination_health_events_legacy_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'destination_health_events_legacy_local', cityHash64(destination_id));
CREATE TABLE IF NOT EXISTS traffoflex.click_events AS traffoflex.click_events_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'click_events_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.conversion_events AS traffoflex.conversion_events_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'conversion_events_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.postback_log_events AS traffoflex.postback_log_events_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'postback_log_events_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.trafficback_events AS traffoflex.trafficback_events_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'trafficback_events_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.destination_health_events AS traffoflex.destination_health_events_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'destination_health_events_local', cityHash64(destination_id));
CREATE TABLE IF NOT EXISTS traffoflex.kafka_ingestion_errors AS traffoflex.kafka_ingestion_errors_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'kafka_ingestion_errors_local', cityHash64(topic));
CREATE TABLE IF NOT EXISTS traffoflex.click_attribution_lookup AS traffoflex.click_attribution_lookup_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'click_attribution_lookup_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.attributed_conversion_events AS traffoflex.attributed_conversion_events_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'attributed_conversion_events_local', cityHash64(click_id));
CREATE TABLE IF NOT EXISTS traffoflex.click_stats_1m AS traffoflex.click_stats_1m_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'click_stats_1m_local', cityHash64(campaign_id));
CREATE TABLE IF NOT EXISTS traffoflex.click_stats_1h AS traffoflex.click_stats_1h_local
ENGINE = Distributed('traffoflex_cluster', 'traffoflex', 'click_stats_1h_local', cityHash64(campaign_id));
SQL
