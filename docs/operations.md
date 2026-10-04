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

The script starts the Docker Compose stack, creates a temporary campaign/stream/destination, verifies click and postback ingestion through Redpanda and ClickHouse, checks reports and ingestion errors, and checks two-user isolation plus administrator editing. It then stops the stack. If host port `27017` is already occupied, the script keeps MongoDB available only inside the Docker network for that run.

## Users and workspaces

Each approved user has a separate workspace. Configuration in MongoDB and analytics in ClickHouse are scoped by `owner_id`; postback secrets identify the owner before a conversion is accepted. A normal user can read and change only their own configuration and reports. An administrator can select an approved user on the **Users** page, then read and edit that user's configuration and reports. The header shows whose workspace is active; **Return to my data** restores the administrator's own workspace. The API checks the administrator role for every `X-TraffoFlex-Act-As` request, and logs the actor ID and owner ID for mutations. `team_id` remains metadata and does not grant cross-user access.

Rejected postbacks for a known network are assigned to its owner. Requests for an unknown network have no reliable owner and remain system-level events.

New users request an email OTP and require administrator approval. Production sessions use an HttpOnly, Secure cookie. Mutating cookie requests require the configured browser origin and `X-TraffoFlex-CSRF: 1`; logout revokes the user's current session generation. Keep `ADMIN_FRONTEND_ORIGIN` set to the exact admin URL.

The owner-aware MongoDB indexes and ClickHouse tables are for fresh volumes. Existing unowned data needs a controlled owner assignment and analytics backfill before this schema is deployed to a populated installation. Docker init scripts alone do not migrate existing volumes.

## Environment

Required settings are documented in `.env.example`.

- `*_SERVICE_ADDR` controls bind addresses inside containers.
- `MONGO_URI` and `MONGO_DATABASE` point to MongoDB config/operational state.
- `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD` and `CLICKHOUSE_DATABASE` configure the local ClickHouse user/database.
- `CLICKHOUSE_HTTP_URL` should include credentials and `database=traffoflex` unless queries are changed to fully qualified table names.
- `EVENT_BROKERS`, `EVENT_BATCH_SIZE`, `EVENT_BATCH_TIMEOUT_MS` and `*_EVENTS_TOPIC` configure Redpanda event publishing.
- `EVENT_WRITE_TIMEOUT_MS`, `EVENT_WRITE_MAX_ATTEMPTS` and `EVENT_WRITE_RETRY_BACKOFF_MS` configure event producer retries. `EVENT_WRITE_FAILURE_POLICY` applies to traffic-service auxiliary event sinks; accepted postbacks always use MongoDB pending delivery regardless of broker availability.
- `DESTINATION_HEALTHCHECK_TIMEOUT_MS` controls outbound destination probe timeout in `traffic-service`.
- `DESTINATION_HEALTHCHECK_INTERVAL_MS` controls periodic destination probe interval in `traffic-service`.
- `DESTINATION_HEALTHCHECK_FAILURES`, `DESTINATION_HEALTHCHECK_RECOVERY` and `DESTINATION_HEALTHCHECK_PARALLELISM` set the consecutive failure/recovery thresholds (defaults: 3/2) and maximum parallel probes (default: 4).
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
- conversion uniqueness by owner, network and transaction ID, plus pending delivery lookup;
- postback log lookup;
- destination health state.

`traffic-service` overlays cached destination health from `destination_health` at startup and upserts the latest probe result after manual or periodic checks. The redirect hot path still reads only in-memory cache.
Health status changes are appended to `destination_health_history` for transition auditing.
The admin UI exposes this history on the Health History screen.

## Incoming postbacks

`postback-service` accepts a valid postback only after inserting a journaled conversion document in MongoDB. The unique `(owner_id, network_id, transaction_id)` index provides idempotency: a repeat returns HTTP 200 with the original conversion ID and creates no second conversion. Each new document starts with `delivery_status: pending`. A background worker sends pending conversions to Redpanda and marks them delivered after a broker acknowledgement. The MongoDB postback log has the same pending/delivered workflow. If MongoDB cannot save or retrieve the conversion, the request returns an error. If Redpanda is unavailable, the accepted conversion remains pending for retry after recovery or process restart. A crash between broker acknowledgement and the MongoDB checkpoint may replay the same `conversion_id`; analytics queries deduplicate it.

No ClickHouse lookup runs while accepting a postback. A separate worker resolves `click_id` against `click_attribution_lookup` in ClickHouse, publishes attributed conversions to Redpanda in batches of up to 100, and marks the MongoDB conversions attributed after broker acknowledgement. Missing or late clicks are retried with a bounded backoff (5 seconds to 5 minutes); the postback response is unaffected. Reports and routing ROI use the compact `attributed_conversion_events` table. Unattributed conversions remain in `conversion_events` and appear in unfiltered totals, but not in campaign or destination totals until attribution succeeds. Monitor the number and age of pending attribution records, Redpanda delivery errors, and ClickHouse ingestion errors.

For the local unauthenticated MongoDB stack, inspect the pending backlog with:

```bash
docker compose -f deploy/docker-compose.yml exec mongo mongosh traffoflex --quiet --eval 'db.conversions.countDocuments({delivery_status: "pending"})'
docker compose -f deploy/docker-compose.yml exec mongo mongosh traffoflex --quiet --eval 'db.postback_logs.countDocuments({delivery_status: "pending"})'
docker compose -f deploy/docker-compose.yml exec mongo mongosh traffoflex --quiet --eval 'db.conversions.countDocuments({attribution_status: "pending"})'
docker compose -f deploy/docker-compose.yml exec mongo mongosh traffoflex --quiet --eval 'db.conversions.find({attribution_status: "pending"}, {created_at: 1, attribution_attempts: 1, attribution_next_attempt_at: 1}).sort({created_at: 1}).limit(5).toArray()'
```

For each destination, set an optional `healthcheck_url` in the admin form to a direct, safe HTTP(S) endpoint that returns 2xx when the destination is usable. It is required when the offer URL contains macros or does not support `HEAD`. Probes use `HEAD` first; only a separately configured healthcheck URL may fall back to `GET` on HTTP 405. Probes never follow redirects, read at most 4 KiB of a response and run outside the click path with bounded parallelism. Without a usable probe URL, the checker leaves the previous health status unchanged and the manual **Check** action shows why. Three consecutive failed probes mark a destination unhealthy; two consecutive successes restore it. Streak counters are in memory and reset after a traffic-service restart or probe URL change. When all destinations are unavailable, normal routing uses the configured trafficback.

Destination schedules are evaluated locally from cached destination config. Caps and ROI ranking use a background ClickHouse snapshot. Anti-repeat checks a local Bloom filter and seeds history from ClickHouse in the background. Check private `/internal/availability/stats` and `/internal/anti-repeat/stats` when routing falls back to trafficback. Keep the traffic-service Bloom volume persistent across restarts.

Campaign routing is restored from the persistent `/data/cache/campaigns.json` snapshot before MongoDB is queried at startup. If MongoDB is unavailable, `/internal/cache/stats` reports `source: snapshot` and the last MongoDB error; the service retries MongoDB in the background. Keep the cache volume persistent. A missing or damaged snapshot cannot provide routes during a MongoDB outage. Active campaigns require a configured trafficback before the persistent cache accepts them.

After a successful campaign, stream or destination change in the admin UI, the frontend calls `POST /api/internal/traffic/cache/reload`. The traffic-service saves the new snapshot before switching its in-memory routes. The admin header also has a manual **Refresh routing snapshot** button. If refresh fails, the UI reports that the MongoDB change was saved while the previous route remains active; the background refresh retries every 30 seconds. API clients outside the admin UI should call the reload endpoint after routing changes or allow the periodic refresh to pick them up.

For existing volumes, Docker init scripts do not re-run automatically. Apply index changes manually with:

```bash
docker compose -f deploy/docker-compose.yml exec mongo mongosh /docker-entrypoint-initdb.d/001_indexes.js
```

## ClickHouse

The MVP schema includes:

- `click_events`
- `conversion_events`
- `click_attribution_lookup`
- `attributed_conversion_events`
- `click_stats_1m`, `click_stats_1h`
- `postback_log_events`
- `trafficback_events`
- `destination_health_events`

The older `clicks`, `conversions` and `postback_logs` tables remain for compatibility with early schema drafts. New application code should write the `*_events` tables.

Application services publish analytics events to Redpanda topics. ClickHouse consumes those topics through Kafka engine queue tables and materialized views from `004_kafka_ingestion.sql`.
The click materialized views in `006_click_rollups.sql` aggregate `click_events` directly by minute, then by hour across campaign, stream, destination and source. Reports use hourly or minute buckets; requested time bounds are rounded outward to whole minutes for both clicks and conversions. Background destination caps and ROI use minute buckets and include the boundary minute, so a cap can trigger up to 59 seconds early. Minute buckets update as clicks arrive; they do not wait for the minute to end. Query rollups with `sum(clicks)` and `sum(cost)` because `SummingMergeTree` merges matching rows asynchronously.
Conversions remain in the compact attributed event table and are deduplicated by `conversion_id` in report queries. At the expected one conversion per second, this avoids a `SummingMergeTree` conversion rollup that would count broker replays twice after an uncertain acknowledgement.
Click reports remain on fast additive minute/hour rollups. `traffic-service` runs a background duplicate audit on startup and every 24 hours over yesterday and today in UTC. If WAL records were recovered at startup, it waits until they are delivered, then audits their full event-date range after a one-minute ingestion grace period and repeats five minutes later. The private `/internal/click-audit/stats` endpoint shows `daily` and `recovery` results, recovered click count, pending recovery check, and failures. Alert on any `duplicate_events`, `conflicting_click_ids`, `error`, or stale `checked_at`. `excess_cost` is the sum of repeated costs after retaining the lowest cost per `click_id`; when costs or attribution fields conflict, investigate the flagged IDs before treating that amount as exact. This audit does not rewrite rollups or slow redirects. If it finds duplicates, inspect the affected period and plan a controlled rebuild of minute/hour rollups before using them for exact billing.
Redpanda auto-create topics is disabled in local compose; `redpanda-init` owns topic creation.

Default event topics:

- `traffoflex.click_events`
- `traffoflex.conversion_events`
- `traffoflex.attributed_conversion_events`
- `traffoflex.postback_log_events`
- `traffoflex.trafficback_events`
- `traffoflex.destination_health_events`

Malformed Kafka messages are stored in `traffoflex.kafka_ingestion_errors`.
The admin UI exposes these rows on the Ingestion screen through `api-service`.
Useful query:

```bash
docker compose -f deploy/docker-compose.yml exec clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex --query "SELECT observed_at, topic, error FROM kafka_ingestion_errors ORDER BY observed_at DESC LIMIT 20"
```

For existing volumes, inspect the old click view chain before applying SQL. If `click_events_to_stats_1s` exists, follow the empty-volume or populated-volume procedure below first; applying `006_click_rollups.sql` alongside the old chain double-counts minute totals. Then apply the needed new SQL manually:

```bash
docker compose -f deploy/docker-compose.yml exec -T clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex < deploy/clickhouse/init/003_mvp_event_tables.sql
docker compose -f deploy/docker-compose.yml exec -T clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex < deploy/clickhouse/init/004_kafka_ingestion.sql
docker compose -f deploy/docker-compose.yml exec -T clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex < deploy/clickhouse/init/005_click_attribution.sql
docker compose -f deploy/docker-compose.yml exec -T clickhouse clickhouse-client --user "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" --database traffoflex < deploy/clickhouse/init/006_click_rollups.sql
```

The new materialized views process only clicks inserted after their creation. On an existing ClickHouse volume with the old second-based views, **do not apply `006_click_rollups.sql` while the old chain is active**: both paths would add the same clicks to `click_stats_1m`. First inspect `SELECT count() FROM click_events`, `click_stats_1m` and `click_stats_1h`. If all three are empty, stop ingestion, drop `click_events_to_stats_1s` and `click_stats_1s_to_stats_1m`, drop the unused `click_stats_1s` table, apply the new `006_click_rollups.sql`, then resume ingestion. If any table has data, preserve it and plan a bounded backfill from `click_events` into the minute rollup while ingestion is paused; verify raw count/cost against both rollups before switching readers. This repository does not run destructive DDL or backfill automatically. Existing MongoDB volumes also need the `conversions_pending_attribution` index from `deploy/mongo/init/001_indexes.js`. Existing conversions without `attribution_status` need a deliberate one-time backfill if they should appear in attributed reports.

On a verified empty volume, run these statements only after stopping click ingestion:

```sql
DROP VIEW IF EXISTS traffoflex.click_events_to_stats_1s;
DROP VIEW IF EXISTS traffoflex.click_stats_1s_to_stats_1m;
DROP TABLE IF EXISTS traffoflex.click_stats_1s;
```

Then apply `006_click_rollups.sql`. The existing `click_stats_1m` and `click_stats_1h` tables remain in place.

The initial expected volume is 10,000–50,000 clicks/day. The earlier 1,000-click/s ceiling would mean 86.4 million clicks/day if sustained; it is a stress target, not the launch forecast. In an isolated synthetic run with one million clicks before rollups were added, `system.parts` reported 141.9 bytes per row in `click_events` and 39.2 bytes per row in `click_attribution_lookup`. Most optional fields were empty, so real traffic, Redpanda retention, logs, MongoDB and backups will use more space. A point lookup took about 5 ms and a join over 86,400 synthetic conversions about 67 ms inside ClickHouse. These are query measurements, not an end-to-end load test. Measure real bytes per click including the minute and hour rollups and set a safe retention and backup policy before scaling traffic on a 180 GB VPS. No automatic ClickHouse TTL is enabled yet.

An earlier isolated run of the retired second-based rollup chain sent one million synthetic clicks across 100 destinations at 1,000 clicks/s. Its results do not validate the new direct minute view. Re-run ingestion and cap benchmarks against the current schema before using those measurements for capacity planning.

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
