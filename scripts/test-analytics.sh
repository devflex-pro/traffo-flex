#!/usr/bin/env bash
# SQL integration and migration checks in an isolated, disposable ClickHouse.
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/.." && pwd)
container="traffoflex-analytics-ci-$$"
port=${ANALYTICS_TEST_PORT:-18125}
work=$(mktemp -d /tmp/traffoflex-analytics-test.XXXXXX)
cleanup() { docker rm -f "$container" >/dev/null; }
trap cleanup EXIT
docker run -d --name "$container" -p "127.0.0.1:$port:8123" --tmpfs /var/lib/clickhouse --tmpfs /var/log/clickhouse-server -e CLICKHOUSE_USER=default -e CLICKHOUSE_PASSWORD=analytics-test-password clickhouse/clickhouse-server:24.8 >/dev/null
for attempt in {1..60}; do
  if docker exec "$container" clickhouse-client --password analytics-test-password --query 'SELECT 1' >/dev/null 2>&1; then break; fi
  sleep 1
done
ch() { docker exec -i "$container" clickhouse-client --password analytics-test-password "$@"; }
{
  printf 'CREATE DATABASE traffoflex_report_test;\n'
  cat "$root/deploy/clickhouse/init/003_mvp_event_tables.sql"
  awk '/^CREATE TABLE IF NOT EXISTS traffoflex.attributed_conversion_events_queue/{exit} {print}' "$root/deploy/clickhouse/init/005_click_attribution.sql"
  cat "$root/deploy/clickhouse/init/006_click_rollups.sql"
} | sed 's/traffoflex\./traffoflex_report_test./g' | ch --multiquery
(cd "$root/apps/api-service" && ANALYTICS_TEST_CLICKHOUSE_URL="http://default:analytics-test-password@127.0.0.1:$port/?database=traffoflex_report_test" go test ./internal/reports -run TestDimensionalReportsOnClickHouse -count=1 -v)
{
  printf 'CREATE DATABASE traffoflex_migration_test;\n'
  cat "$root/deploy/clickhouse/init/003_mvp_event_tables.sql" "$root/deploy/clickhouse/tests/legacy_rollups.sql"
  printf "INSERT INTO traffoflex.click_events (created_at,owner_id,click_id,campaign_id,sub1,cost,query) VALUES ('2026-01-01 01:20:30','owner','a','campaign','legacy-zone',0.2,'{}'),('2026-01-02 23:50:00','owner','b','campaign','',0.3,'{\"zone_id\":\"named-zone\"}');\n"
} | sed 's/traffoflex\./traffoflex_migration_test./g' | ch --multiquery
bash "$root/scripts/migrate-hour-day-rollups.sh" --container "$container" --database traffoflex_migration_test --backup-dir "$work/schema"
bash "$root/scripts/migrate-hour-day-rollups.sh" --container "$container" --database traffoflex_migration_test
[[ $(ch --query "SELECT sum(clicks) FROM traffoflex_migration_test.click_stats_1d") == 2 ]]
[[ $(ch --query "SELECT count() FROM traffoflex_migration_test.click_attribution_lookup WHERE zone_id IN ('legacy-zone','named-zone')") == 2 ]]
ch --query "INSERT INTO traffoflex_migration_test.click_events (created_at,owner_id,click_id,cost) VALUES ('2026-01-03 00:00:00','owner','rollback-new-click',0.1)"
bash "$root/scripts/migrate-hour-day-rollups.sh" --restore --backup-dir "$work/schema"
[[ $(ch --query "SELECT sum(clicks) FROM traffoflex_migration_test.click_stats_1m") == 3 ]]
bash "$root/scripts/migrate-hour-day-rollups.sh" --container "$container" --database traffoflex_migration_test --backup-dir "$work/schema-second"
ch --query "INSERT INTO traffoflex_migration_test.click_events (created_at,owner_id,click_id,cost) VALUES ('2026-01-03 00:00:01','owner','live',0.5)"
[[ $(ch --query "SELECT sum(clicks) FROM traffoflex_migration_test.click_stats_1d") == 4 ]]
# Incomplete raw retention must never silently erase aggregate history.
ch --query "CREATE DATABASE traffoflex_mismatch_test"
sed 's/traffoflex\./traffoflex_mismatch_test./g' "$root/deploy/clickhouse/init/003_mvp_event_tables.sql" "$root/deploy/clickhouse/tests/legacy_rollups.sql" | ch --multiquery
ch --query "INSERT INTO traffoflex_mismatch_test.click_stats_1h (created_at,owner_id,clicks,cost) VALUES ('2026-01-01','owner',5,0.5)"
if bash "$root/scripts/migrate-hour-day-rollups.sh" --container "$container" --database traffoflex_mismatch_test --backup-dir "$work/mismatch"; then printf 'Incomplete raw history was accepted\n' >&2; exit 1; fi
[[ $(ch --query "SELECT sum(clicks) FROM traffoflex_mismatch_test.click_stats_1h") == 5 ]]
printf 'Analytics integration and migration checks passed.\n'
