#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILES=(-f deploy/docker-compose.yml)
COMPOSE_ENV=(--env-file .env.example)
API_URL="${API_URL:-http://localhost:8070}"
TRAFFIC_URL="${TRAFFIC_URL:-http://localhost:8080}"
POSTBACK_URL="${POSTBACK_URL:-http://localhost:8081}"
ADMIN_URL="${ADMIN_URL:-http://localhost:5173}"
ADMIN_EMAIL="${AUTH_ADMIN_EMAIL:-admin@example.com}"
CLICKHOUSE_USER="${CLICKHOUSE_USER:-default}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-traffoflex}"
CLICKHOUSE_DATABASE="${CLICKHOUSE_DATABASE:-traffoflex}"
RUN_ID="${RUN_ID:-$(date +%s)}"
COMPOSE_PROJECT_NAME="traffoflex-smoke-${RUN_ID}"
export COMPOSE_PROJECT_NAME
SLUG="e2e-${RUN_ID}"
TRANSACTION_ID="tx_e2e_${RUN_ID}"
POSTBACK_SECRET="smoke_${RUN_ID}"
TMP_DIR="$(mktemp -d)"
CREATED_ENV=0
AUTH_TOKEN=""
ACT_AS_USER_ID=""

cd "$ROOT_DIR"

cleanup() {
  local status=$?
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    docker compose \
      "${COMPOSE_ENV[@]}" \
      "${COMPOSE_FILES[@]}" \
      down \
      -v
  fi
  if [[ "$CREATED_ENV" == "1" ]]; then
    rm -f .env
  fi
  rm -rf "$TMP_DIR"
  exit "$status"
}
trap cleanup EXIT

log() {
  printf '[e2e] %s\n' "$*"
}

fail() {
  printf '[e2e] ERROR: %s\n' "$*" >&2
  exit 1
}

json_field() {
  local file=$1
  local field=$2
  python3 -c 'import json, sys
value = json.load(open(sys.argv[1]))
for key in sys.argv[2].split("."):
    value = value[key]
print(value)' "$file" "$field"
}

http_json() {
  local method=$1
  local url=$2
  local body=${3:-}
  local output=$4
  local status
  local auth_args=()

  if [[ -n "$AUTH_TOKEN" ]]; then
    auth_args=(-H "Authorization: Bearer $AUTH_TOKEN")
  fi
  if [[ -n "$ACT_AS_USER_ID" ]]; then
    auth_args+=(-H "X-TraffoFlex-Act-As: $ACT_AS_USER_ID")
  fi

  if [[ -n "$body" ]]; then
    status="$(curl \
      -sS \
      -o "$output" \
      -w "%{http_code}" \
      -X "$method" \
      -H "Content-Type: application/json" \
      "${auth_args[@]}" \
      --data "$body" \
      "$url")"
  else
    status="$(curl \
      -sS \
      -o "$output" \
      -w "%{http_code}" \
      -X "$method" \
      "${auth_args[@]}" \
      "$url")"
  fi

  case "$status" in
    2*) ;;
    *) fail "$method $url returned HTTP $status: $(cat "$output")" ;;
  esac
}

wait_http() {
  local url=$1
  local expected=$2
  local name=$3

  for _ in $(seq 1 90); do
    if curl \
      -fs \
      "$url" \
      | grep \
        -q \
        "$expected"; then
      log "$name is ready"
      return
    fi
    sleep 2
  done

  fail "$name did not become ready"
}

clickhouse_query() {
  local query=$1
  docker compose \
    "${COMPOSE_ENV[@]}" \
    "${COMPOSE_FILES[@]}" \
    exec \
    -T \
    clickhouse \
    clickhouse-client \
    --user "$CLICKHOUSE_USER" \
    --password "$CLICKHOUSE_PASSWORD" \
    --database "$CLICKHOUSE_DATABASE" \
    --query "$query"
}

wait_clickhouse_row() {
  local query=$1
  local output=$2
  local name=$3

  for _ in $(seq 1 60); do
    if clickhouse_query "$query" > "$output" && [[ -s "$output" ]]; then
      log "$name found in ClickHouse"
      return
    fi
    sleep 2
  done

  fail "$name was not found in ClickHouse"
}

if [[ ! -f .env ]]; then
  cp .env.example .env
  CREATED_ENV=1
fi

if ss -ltn 2>/dev/null | grep -q ':27017 '; then
  cat > "$TMP_DIR/docker-compose.override.yml" <<'YAML'
services:
  mongo:
    ports: !reset []
YAML
  COMPOSE_FILES+=(-f "$TMP_DIR/docker-compose.override.yml")
  log "host port 27017 is busy; MongoDB host port will not be published"
fi

log "starting stack"
docker compose \
  "${COMPOSE_ENV[@]}" \
  "${COMPOSE_FILES[@]}" \
  up \
  --build \
  -d

wait_http "$API_URL/readyz" "ready" "api-service"
wait_http "$TRAFFIC_URL/healthz" "ok" "traffic-service"
wait_http "$POSTBACK_URL/readyz" "ready" "postback-service"
wait_http "$ADMIN_URL/" "TraffoFlex" "admin-frontend"

log "authenticating admin"
http_json \
  POST \
  "$API_URL/api/auth/request-otp" \
  "$(printf '{"email":"%s"}' "$ADMIN_EMAIL")" \
  "$TMP_DIR/auth-request.json"
ADMIN_OTP="$(json_field "$TMP_DIR/auth-request.json" otp)"
if [[ -z "$ADMIN_OTP" ]]; then
  fail "admin otp is empty; set AUTH_DEV_RETURN_OTP=true for local smoke"
fi
http_json \
  POST \
  "$API_URL/api/auth/verify-otp" \
  "$(printf '{"email":"%s","otp":"%s"}' "$ADMIN_EMAIL" "$ADMIN_OTP")" \
  "$TMP_DIR/auth-session.json"
AUTH_TOKEN="$(json_field "$TMP_DIR/auth-session.json" token)"
ADMIN_TOKEN="$AUTH_TOKEN"

http_json \
  POST \
  "$API_URL/api/traffic-sources" \
  "$(printf '{"name":"E2E Source %s","slug":"e2e-source-%s"}' "$RUN_ID" "$RUN_ID")" \
  "$TMP_DIR/source.json"
SOURCE_ID="$(json_field "$TMP_DIR/source.json" id)"
http_json \
  POST \
  "$API_URL/api/affiliate-networks" \
  "$(printf '{"name":"E2E Network %s","slug":"e2e-network-%s"}' "$RUN_ID" "$RUN_ID")" \
  "$TMP_DIR/network.json"
NETWORK_ID="$(json_field "$TMP_DIR/network.json" id)"

http_json \
  POST \
  "$API_URL/api/postback-templates" \
  "$(printf '{"network_id":"%s","name":"E2E Postback %s","slug":"e2e-%s","secret":"%s","mapping":{"click_id":"cid"}}' "$NETWORK_ID" "$RUN_ID" "$RUN_ID" "$POSTBACK_SECRET")" \
  "$TMP_DIR/postback-template.json"

DESTINATION_BODY="$(printf '{"name":"E2E Destination %s","type":"url","url":"http://api-service:8070/healthz?cid={click_id}","manual_status":"active","health_status":"healthy","redirect":{"mode":"http_302"}}' "$RUN_ID")"
CAMPAIGN_BODY="$(printf '{"name":"E2E Campaign %s","slug":"%s","status":"active","traffic_source_id":"%s","trafficback_config":{"enabled":true,"url":"http://api-service:8070/healthz","max_depth":3}}' "$RUN_ID" "$SLUG" "$SOURCE_ID")"

http_json \
  POST \
  "$API_URL/api/destinations" \
  "$DESTINATION_BODY" \
  "$TMP_DIR/destination.json"
DESTINATION_ID="$(json_field "$TMP_DIR/destination.json" id)"

http_json \
  POST \
  "$API_URL/api/campaigns" \
  "$CAMPAIGN_BODY" \
  "$TMP_DIR/campaign.json"
CAMPAIGN_ID="$(json_field "$TMP_DIR/campaign.json" id)"

STREAM_BODY="$(printf '{"name":"E2E Stream %s","priority":0,"status":"active","conditions":[],"distribution":{"mode":"weighted","destinations":[{"destination_id":"%s","weight":100}]}}' "$RUN_ID" "$DESTINATION_ID")"
http_json \
  POST \
  "$API_URL/api/campaigns/$CAMPAIGN_ID/streams" \
  "$STREAM_BODY" \
  "$TMP_DIR/stream.json"
STREAM_ID="$(json_field "$TMP_DIR/stream.json" id)"

http_json \
  POST \
  "$API_URL/api/internal/traffic/cache/reload" \
  "" \
  "$TMP_DIR/reload.json"
wait_http "$TRAFFIC_URL/readyz" "ready" "traffic-service"

log "executing click flow"
CLICK_HEADERS="$TMP_DIR/click.headers"
CLICK_STATUS="$(curl \
  -sS \
  -o "$TMP_DIR/click.body" \
  -D "$CLICK_HEADERS" \
  -w "%{http_code}" \
  "$TRAFFIC_URL/c/$SLUG?sub1=zone_a&cost=1.25&currency=USD")"
if [[ "$CLICK_STATUS" != "302" ]]; then
  fail "click returned HTTP $CLICK_STATUS: $(cat "$TMP_DIR/click.body")"
fi
LOCATION="$(awk 'tolower($1) == "location:" {print $2}' "$CLICK_HEADERS" | tr -d '\r')"
CLICK_ID="$(python3 -c 'import sys, urllib.parse
parsed = urllib.parse.urlparse(sys.argv[1])
print(urllib.parse.parse_qs(parsed.query)["cid"][0])' "$LOCATION")"

wait_clickhouse_row \
  "SELECT click_id, campaign_id, stream_id, destination_id, source_id, cost, currency FROM click_events WHERE click_id = '$CLICK_ID' FORMAT JSONEachRow" \
  "$TMP_DIR/click-event.json" \
  "click event"
wait_clickhouse_row \
  "SELECT click_id FROM click_attribution_lookup WHERE click_id = '$CLICK_ID' FORMAT JSONEachRow" \
  "$TMP_DIR/click-attribution.json" \
  "click attribution lookup"
for granularity in 1m 1h; do
  wait_clickhouse_row \
    "SELECT campaign_id, sum(clicks) AS clicks, sum(cost) AS cost FROM click_stats_$granularity WHERE campaign_id = '$CAMPAIGN_ID' GROUP BY campaign_id HAVING clicks = 1 FORMAT JSONEachRow" \
    "$TMP_DIR/click-stats-$granularity.json" \
    "click stats $granularity"
done

python3 -c 'import json, sys
row = json.load(open(sys.argv[1]))
expected = {
    "campaign_id": sys.argv[2],
    "stream_id": sys.argv[3],
    "destination_id": sys.argv[4],
    "source_id": sys.argv[5],
    "currency": "USD",
}
for key, value in expected.items():
    if row.get(key) != value:
        raise SystemExit(f"{key}={row.get(key)!r}, want {value!r}")
if float(row.get("cost")) != 1.25:
    raise SystemExit(f"cost={row.get('cost')!r}, want 1.25")' \
  "$TMP_DIR/click-event.json" \
  "$CAMPAIGN_ID" \
  "$STREAM_ID" \
  "$DESTINATION_ID" \
  "$SOURCE_ID"

log "executing postback flow"
http_json \
  GET \
  "$POSTBACK_URL/pb/$NETWORK_ID?cid=$CLICK_ID&tx=$TRANSACTION_ID&sum=7.77&currency=USD&status=approved&secret=$POSTBACK_SECRET" \
  "" \
  "$TMP_DIR/postback.json"
CONVERSION_ID="$(json_field "$TMP_DIR/postback.json" conversion_id)"
POSTBACK_ID="$(json_field "$TMP_DIR/postback.json" postback_id)"

wait_clickhouse_row \
  "SELECT conversion_id, click_id, transaction_id, payout, currency, network_id FROM conversion_events WHERE conversion_id = '$CONVERSION_ID' FORMAT JSONEachRow" \
  "$TMP_DIR/conversion-event.json" \
  "conversion event"
wait_clickhouse_row \
  "SELECT conversion_id, campaign_id, stream_id, destination_id, source_id FROM attributed_conversion_events WHERE conversion_id = '$CONVERSION_ID' FORMAT JSONEachRow" \
  "$TMP_DIR/attributed-conversion.json" \
  "attributed conversion event"
wait_clickhouse_row \
  "SELECT postback_id FROM postback_log_events WHERE postback_id = '$POSTBACK_ID' FORMAT JSONEachRow" \
  "$TMP_DIR/postback-log-event.json" \
  "postback log event"

python3 -c 'import json, sys
row = json.load(open(sys.argv[1]))
expected = {
    "click_id": sys.argv[2],
    "transaction_id": sys.argv[3],
    "currency": "USD",
    "network_id": sys.argv[4],
}
for key, value in expected.items():
    if row.get(key) != value:
        raise SystemExit(f"{key}={row.get(key)!r}, want {value!r}")
if float(row.get("payout")) != 7.77:
    raise SystemExit(f"payout={row.get('payout')!r}, want 7.77")' \
  "$TMP_DIR/conversion-event.json" \
  "$CLICK_ID" \
  "$TRANSACTION_ID" \
  "$NETWORK_ID"

http_json \
  GET \
  "$POSTBACK_URL/pb/$NETWORK_ID?cid=$CLICK_ID&tx=$TRANSACTION_ID&sum=7.77&currency=USD&status=approved&secret=$POSTBACK_SECRET" \
  "" \
  "$TMP_DIR/postback-duplicate.json"
if [[ "$(json_field "$TMP_DIR/postback-duplicate.json" conversion_id)" != "$CONVERSION_ID" ]]; then
  fail "duplicate postback returned a different conversion ID"
fi

http_json \
  GET \
  "$API_URL/api/reports/overview?campaign_id=$CAMPAIGN_ID" \
  "" \
  "$TMP_DIR/overview.json"
python3 -c 'import json, sys
overview = json.load(open(sys.argv[1]))
if overview.get("clicks") != 1:
    raise SystemExit(f"clicks={overview.get('clicks')!r}, want 1")
if overview.get("conversions") != 1:
    raise SystemExit(f"conversions={overview.get('conversions')!r}, want 1")
if float(overview.get("revenue")) != 7.77:
    raise SystemExit(f"revenue={overview.get('revenue')!r}, want 7.77")
if float(overview.get("cost")) != 1.25:
    raise SystemExit(f"cost={overview.get('cost')!r}, want 1.25")' "$TMP_DIR/overview.json"

log "verifying user isolation and admin editing"
USER_EMAIL="e2e-user-${RUN_ID}@example.com"
http_json \
  POST \
  "$API_URL/api/auth/request-otp" \
  "$(printf '{"email":"%s"}' "$USER_EMAIL")" \
  "$TMP_DIR/user-otp.json"
USER_OTP="$(json_field "$TMP_DIR/user-otp.json" otp)"
http_json GET "$API_URL/api/users" "" "$TMP_DIR/users.json"
USER_ID="$(python3 -c 'import json, sys
users = json.load(open(sys.argv[1]))["items"]
print(next(user["id"] for user in users if user["email"] == sys.argv[2]))' "$TMP_DIR/users.json" "$USER_EMAIL")"
http_json POST "$API_URL/api/users/$USER_ID/approve" "" "$TMP_DIR/user-approved.json"
http_json \
  POST \
  "$API_URL/api/auth/verify-otp" \
  "$(printf '{"email":"%s","otp":"%s"}' "$USER_EMAIL" "$USER_OTP")" \
  "$TMP_DIR/user-session.json"
AUTH_TOKEN="$(json_field "$TMP_DIR/user-session.json" token)"
http_json GET "$API_URL/api/campaigns" "" "$TMP_DIR/user-campaigns.json"
python3 -c 'import json, sys
items = json.load(open(sys.argv[1]))["items"]
assert items == [], items' "$TMP_DIR/user-campaigns.json"
http_json GET "$API_URL/api/reports/overview" "" "$TMP_DIR/user-overview.json"
python3 -c 'import json, sys
report = json.load(open(sys.argv[1]))
assert report["clicks"] == 0 and report["conversions"] == 0, report' "$TMP_DIR/user-overview.json"
USER_GET_STATUS="$(curl -sS -o "$TMP_DIR/user-campaign-get.json" -w "%{http_code}" -H "Authorization: Bearer $AUTH_TOKEN" "$API_URL/api/campaigns/$CAMPAIGN_ID")"
[[ "$USER_GET_STATUS" == "404" ]] || fail "user read another user's campaign: HTTP $USER_GET_STATUS"
USER_ACT_AS_STATUS="$(curl -sS -o "$TMP_DIR/user-act-as.json" -w "%{http_code}" -H "Authorization: Bearer $AUTH_TOKEN" -H "X-TraffoFlex-Act-As: $(json_field "$TMP_DIR/auth-session.json" user.id)" "$API_URL/api/campaigns")"
[[ "$USER_ACT_AS_STATUS" == "403" ]] || fail "user impersonation returned HTTP $USER_ACT_AS_STATUS"

AUTH_TOKEN="$ADMIN_TOKEN"
ACT_AS_USER_ID="$USER_ID"
http_json \
  POST \
  "$API_URL/api/destinations" \
  "$(printf '{"name":"User Destination %s","type":"url","url":"https://example.com/?cid={click_id}","manual_status":"active","health_status":"healthy"}' "$RUN_ID")" \
  "$TMP_DIR/user-destination.json"
USER_DESTINATION_ID="$(json_field "$TMP_DIR/user-destination.json" id)"
http_json \
  PUT \
  "$API_URL/api/destinations/$USER_DESTINATION_ID" \
  "$(printf '{"name":"Edited by Admin %s","type":"url","url":"https://example.com/?cid={click_id}","manual_status":"active","health_status":"healthy"}' "$RUN_ID")" \
  "$TMP_DIR/user-destination-updated.json"
ACT_AS_USER_ID=""
AUTH_TOKEN="$(json_field "$TMP_DIR/user-session.json" token)"
http_json GET "$API_URL/api/destinations/$USER_DESTINATION_ID" "" "$TMP_DIR/user-destination-read.json"
python3 -c 'import json, sys
item = json.load(open(sys.argv[1]))
assert item["name"].startswith("Edited by Admin"), item' "$TMP_DIR/user-destination-read.json"
AUTH_TOKEN="$ADMIN_TOKEN"
ADMIN_GET_STATUS="$(curl -sS -o "$TMP_DIR/admin-own-destination.json" -w "%{http_code}" -H "Authorization: Bearer $AUTH_TOKEN" "$API_URL/api/destinations/$USER_DESTINATION_ID")"
[[ "$ADMIN_GET_STATUS" == "404" ]] || fail "admin's own workspace exposed another user's data: HTTP $ADMIN_GET_STATUS"

http_json \
  GET \
  "$TRAFFIC_URL/internal/click-audit/stats" \
  "" \
  "$TMP_DIR/click-audit.json"
python3 -c 'import json, sys
stats = json.load(open(sys.argv[1]))
daily = stats["daily"]
if daily.get("error") or daily.get("checked_at", "").startswith("0001-"):
    raise SystemExit(f"click audit did not complete: {stats}")' "$TMP_DIR/click-audit.json"

clickhouse_query \
  "SELECT count() AS errors FROM kafka_ingestion_errors FORMAT JSONEachRow" \
  > "$TMP_DIR/ingestion-errors.json"
python3 -c 'import json, sys
row = json.load(open(sys.argv[1]))
if int(row.get("errors")) != 0:
    raise SystemExit(f"errors={row.get('errors')!r}, want 0")' "$TMP_DIR/ingestion-errors.json"

docker compose \
  "${COMPOSE_ENV[@]}" \
  "${COMPOSE_FILES[@]}" \
  exec \
  -T \
  redpanda-0 \
  rpk \
  topic \
  list \
  -X \
  brokers=redpanda-0:9092,redpanda-1:9092,redpanda-2:9092 \
  > "$TMP_DIR/topics.txt"

for topic in \
  traffoflex.click_events \
  traffoflex.conversion_events \
  traffoflex.attributed_conversion_events \
  traffoflex.destination_health_events \
  traffoflex.postback_log_events \
  traffoflex.trafficback_events
do
  grep \
    -q \
    "$topic" \
    "$TMP_DIR/topics.txt" \
    || fail "missing Redpanda topic $topic"
done

log "verifying duplicate-click audit"
clickhouse_query \
  "INSERT INTO click_events SELECT * FROM click_events WHERE click_id = '$CLICK_ID'"
docker compose \
  "${COMPOSE_ENV[@]}" \
  "${COMPOSE_FILES[@]}" \
  restart \
  traffic-service
wait_http "$TRAFFIC_URL/readyz" "ready" "traffic-service after restart"
AUDIT_FOUND=0
for _ in $(seq 1 30); do
  http_json \
    GET \
    "$TRAFFIC_URL/internal/click-audit/stats" \
    "" \
    "$TMP_DIR/click-audit-duplicate.json"
  if python3 -c 'import json, sys
row = json.load(open(sys.argv[1]))["daily"]
assert row["duplicate_events"] == 1, row
assert abs(row["excess_cost"] - 1.25) < 0.000001, row
assert row["conflicting_click_ids"] == 0, row' "$TMP_DIR/click-audit-duplicate.json" >/dev/null 2>&1; then
    AUDIT_FOUND=1
    break
  fi
  sleep 2
done
if [[ "$AUDIT_FOUND" != "1" ]]; then
  fail "duplicate-click audit did not report one repeated click: $(cat "$TMP_DIR/click-audit-duplicate.json")"
fi

log "smoke passed: click_id=$CLICK_ID conversion_id=$CONVERSION_ID campaign_id=$CAMPAIGN_ID"
