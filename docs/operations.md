# Operations

## Startup

Local self-hosted startup:

```bash
cp .env.example .env
docker compose -f deploy/docker-compose.yml up --build
```

Startup order is enforced by Docker healthchecks:

1. Three Redpanda brokers start as the durable Kafka-compatible event log.
2. `redpanda-init` creates required topics with 12 partitions, replication factor 3 and 7 day retention.
3. MongoDB starts and runs `deploy/mongo/init/*.js` for indexes on first volume initialization.
4. ClickHouse starts and runs `deploy/clickhouse/init/*.sql` for schemas on first volume initialization.
5. Backend services wait for MongoDB, ClickHouse and, where needed, Redpanda healthchecks.
6. `admin-frontend` waits for `api-service`.

The services expose:

- `GET /healthz`: process liveness.
- `GET /readyz`: config/dependency readiness contract.

Redpanda Console is exposed at `http://localhost:8085`.

## E2E smoke

Run the repeatable local smoke check after backend, pipeline or report changes:

```bash
make e2e-smoke
```

The script starts the Docker Compose stack, creates a temporary campaign/stream/destination, verifies click and postback ingestion through Redpanda and ClickHouse, checks reports and ingestion errors, then stops the stack. If host port `27017` is already occupied, the script keeps MongoDB available only inside the Docker network for that run.

## Environment

Required settings are documented in `.env.example`.

- `*_SERVICE_ADDR` controls bind addresses inside containers.
- `MONGO_URI` and `MONGO_DATABASE` point to MongoDB config/operational state.
- `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD` and `CLICKHOUSE_DATABASE` configure the local ClickHouse user/database.
- `CLICKHOUSE_HTTP_URL` should include credentials and `database=traffoflex` unless queries are changed to fully qualified table names.
- `EVENT_BROKERS`, `EVENT_BATCH_SIZE`, `EVENT_BATCH_TIMEOUT_MS` and `*_EVENTS_TOPIC` configure Redpanda event publishing.
- `EVENT_WRITE_TIMEOUT_MS`, `EVENT_WRITE_MAX_ATTEMPTS`, `EVENT_WRITE_RETRY_BACKOFF_MS` and `EVENT_WRITE_FAILURE_POLICY` configure event producer retry behavior and fail-open/fail-closed handling.
- `CLICK_LOOKUP_TIMEOUT_MS` and `CLICK_LOOKUP_FAILURE_POLICY` configure postback click enrichment from ClickHouse.
- `DESTINATION_HEALTHCHECK_TIMEOUT_MS` controls outbound destination probe timeout in `traffic-service`.
- `DESTINATION_HEALTHCHECK_INTERVAL_MS` controls periodic destination probe interval in `traffic-service`.
- `DESTINATION_CAP_CACHE_TTL_MS` controls how long `traffic-service` caches destination cap, unique history and ROI ranking checks from ClickHouse.
- `TRAFFIC_SERVICE_URL` and `POSTBACK_SERVICE_URL` are internal URLs used by `api-service`.
- `ADMIN_FRONTEND_ORIGIN` must match the browser origin for CORS.
- `AUTH_ADMIN_EMAIL` is the built-in admin email. This user is auto-approved after OTP verification.
- `AUTH_JWT_SECRET`, `AUTH_OTP_TTL_SECONDS`, `AUTH_OTP_RATE_LIMIT_MINUTES` and `AUTH_SESSION_TTL_HOURS` configure JWT sessions, OTP lifetime and OTP request throttling. `AUTH_OTP_TTL_MINUTES` is still accepted as a legacy fallback when seconds are not set. A new OTP is not sent while the previous code is still valid.
- `AUTH_ENV` controls auth safety mode. `AUTH_DEV_RETURN_OTP=true` is accepted only for `local`, `dev` or `test`.
- `AUTH_EMAIL_FROM`, `AUTH_RESEND_API_KEY`, `AUTH_RESEND_API_URL`, `AUTH_RESEND_MAX_ATTEMPTS` and `AUTH_RESEND_RETRY_BACKOFF_MS` configure production OTP email delivery through Resend. A Resend API key is required outside local/dev/test auth environments.
- `TRUSTED_PROXY_CIDRS` must stay empty unless traffic is behind known proxies.

## MongoDB

The init script creates indexes for:

- campaign slug/public identifiers;
- stream lookup by campaign;
- traffic source and affiliate network slugs;
- postback templates by network and slug;
- conversion dedupe by network and transaction ID;
- postback log lookup;
- destination health state.

`traffic-service` overlays cached destination health from `destination_health` at startup and upserts the latest probe result after manual or periodic checks. The redirect hot path still reads only in-memory cache.
Health status changes are appended to `destination_health_history` for transition auditing.
The admin UI exposes this history on the Health History screen.

Destination schedules are evaluated locally from cached destination config. Destination caps, anti-repeat history checks and ROI ranking are checked against ClickHouse with a short TTL cache so the redirect hot path does not query ClickHouse on every click.

For existing volumes, Docker init scripts do not re-run automatically. Apply index changes manually with:

```bash
docker compose -f deploy/docker-compose.yml exec mongo mongosh /docker-entrypoint-initdb.d/001_indexes.js
```

## ClickHouse

The MVP schema includes:

- `click_events`
- `conversion_events`
- `postback_log_events`
- `trafficback_events`
- `destination_health_events`

The older `clicks`, `conversions` and `postback_logs` tables remain for compatibility with early schema drafts. New application code should write the `*_events` tables.

Application services publish analytics events to Redpanda topics. ClickHouse consumes those topics through Kafka engine queue tables and materialized views from `004_kafka_ingestion.sql`.
Redpanda auto-create topics is disabled in local compose; `redpanda-init` owns topic creation.

Default event topics:

- `traffoflex.click_events`
- `traffoflex.conversion_events`
- `traffoflex.postback_log_events`
- `traffoflex.trafficback_events`
- `traffoflex.destination_health_events`

Malformed Kafka messages are stored in `traffoflex.kafka_ingestion_errors`.
The admin UI exposes these rows on the Ingestion screen through `api-service`.
Useful query:

```bash
docker compose -f deploy/docker-compose.yml exec clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex --query "SELECT observed_at, topic, error FROM kafka_ingestion_errors ORDER BY observed_at DESC LIMIT 20"
```

For existing volumes, apply new SQL manually:

```bash
docker compose -f deploy/docker-compose.yml exec -T clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex < deploy/clickhouse/init/003_mvp_event_tables.sql
docker compose -f deploy/docker-compose.yml exec -T clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex < deploy/clickhouse/init/004_kafka_ingestion.sql
```

If `conversion_events` already exists without `source_id`, apply the additive column and recreate the Kafka ingestion objects before restarting ingestion:

```bash
docker compose -f deploy/docker-compose.yml exec clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex --query "ALTER TABLE conversion_events ADD COLUMN IF NOT EXISTS source_id String AFTER destination_id"
docker compose -f deploy/docker-compose.yml exec clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex --query "DROP VIEW IF EXISTS conversion_events_mv"
docker compose -f deploy/docker-compose.yml exec clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex --query "DROP TABLE IF EXISTS conversion_events_queue"
docker compose -f deploy/docker-compose.yml exec -T clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex < deploy/clickhouse/init/004_kafka_ingestion.sql
```

## Backup

MongoDB backup:

```bash
docker compose -f deploy/docker-compose.yml exec mongo mongodump --archive=/tmp/traffoflex-mongo.archive --db traffoflex
docker compose -f deploy/docker-compose.yml cp mongo:/tmp/traffoflex-mongo.archive ./traffoflex-mongo.archive
```

MongoDB restore:

```bash
docker compose -f deploy/docker-compose.yml cp ./traffoflex-mongo.archive mongo:/tmp/traffoflex-mongo.archive
docker compose -f deploy/docker-compose.yml exec mongo mongorestore --archive=/tmp/traffoflex-mongo.archive --drop
```

ClickHouse backup should be volume-level or table export based until production backup tooling is selected. Minimal table export example:

```bash
docker compose -f deploy/docker-compose.yml exec clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex --query "SELECT * FROM click_events FORMAT Native" > click_events.native
```

## Observability

Backend services write structured JSON logs with:

- `service`
- `request_id`
- HTTP method/path/status/latency;
- contextual error fields.

`traffic-service` and `postback-service` expose event producer counters at:

```http
GET /internal/event-producer/stats
```

The response includes write calls, Kafka write attempts, successful writes, final failures, retry count, marshal/context failures, bytes written and last success/failure timestamps. Event write logs also include cumulative producer counters.

Useful checks:

```bash
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs redpanda-0
docker compose -f deploy/docker-compose.yml logs redpanda-1
docker compose -f deploy/docker-compose.yml logs redpanda-2
docker compose -f deploy/docker-compose.yml logs api-service
docker compose -f deploy/docker-compose.yml logs traffic-service
docker compose -f deploy/docker-compose.yml logs postback-service
```

Operational failures to watch:

- ClickHouse write/query failures in analytics and report endpoints.
- Rising `write_failure_total`, `write_retry_total` or `context_cancel_total` from event producer stats.
- Rising destination cap check warnings in `traffic-service` logs.
- Rows in `kafka_ingestion_errors`.
- Postback secret validation errors.
- Duplicate conversion dedupe hits.
- Outbound postback retry failures.
- Destination healthcheck failures.
- Trafficback loop protection hits.
