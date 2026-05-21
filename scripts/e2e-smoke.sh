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
SLUG="e2e-${RUN_ID}"
SOURCE_ID="src_e2e_${RUN_ID}"
TRANSACTION_ID="tx_e2e_${RUN_ID}"
TMP_DIR="$(mktemp -d)"
CREATED_ENV=0
AUTH_TOKEN=""

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
print(json.load(open(sys.argv[1]))[sys.argv[2]])' "$file" "$field"
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
wait_http "$TRAFFIC_URL/readyz" "ready" "traffic-service"
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

DESTINATION_BODY="$(printf '{"name":"E2E Destination %s","type":"url","url":"http://api-service:8070/healthz?cid={click_id}","manual_status":"active","health_status":"healthy","redirect":{"mode":"http_302"}}' "$RUN_ID")"
CAMPAIGN_BODY="$(printf '{"name":"E2E Campaign %s","slug":"%s","status":"active","traffic_source_id":"%s"}' "$RUN_ID" "$SLUG" "$SOURCE_ID")"

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
  "$POSTBACK_URL/pb/e2e-net?cid=$CLICK_ID&tx=$TRANSACTION_ID&sum=7.77&currency=USD&status=approved" \
  "" \
  "$TMP_DIR/postback.json"
CONVERSION_ID="$(json_field "$TMP_DIR/postback.json" conversion_id)"

wait_clickhouse_row \
  "SELECT conversion_id, click_id, transaction_id, campaign_id, stream_id, destination_id, source_id, payout, currency, network_id FROM conversion_events WHERE conversion_id = '$CONVERSION_ID' FORMAT JSONEachRow" \
  "$TMP_DIR/conversion-event.json" \
  "conversion event"

python3 -c 'import json, sys
row = json.load(open(sys.argv[1]))
expected = {
    "click_id": sys.argv[2],
    "transaction_id": sys.argv[3],
    "campaign_id": sys.argv[4],
    "stream_id": sys.argv[5],
    "destination_id": sys.argv[6],
    "source_id": sys.argv[7],
    "currency": "USD",
    "network_id": "e2e-net",
}
for key, value in expected.items():
    if row.get(key) != value:
        raise SystemExit(f"{key}={row.get(key)!r}, want {value!r}")
if float(row.get("payout")) != 7.77:
    raise SystemExit(f"payout={row.get('payout')!r}, want 7.77")' \
  "$TMP_DIR/conversion-event.json" \
  "$CLICK_ID" \
  "$TRANSACTION_ID" \
  "$CAMPAIGN_ID" \
  "$STREAM_ID" \
  "$DESTINATION_ID" \
  "$SOURCE_ID"

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

log "smoke passed: click_id=$CLICK_ID conversion_id=$CONVERSION_ID campaign_id=$CAMPAIGN_ID"
