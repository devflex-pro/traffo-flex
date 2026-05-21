# Analytics

`api-service` exposes MVP reports through `/api/reports/*` and reads metrics from ClickHouse through `CLICKHOUSE_HTTP_URL`.

## API reports

- `GET /api/reports/overview`
- `GET /api/reports/campaigns`
- `GET /api/reports/streams`
- `GET /api/reports/destinations`
- `GET /api/reports/sources`
- `GET /api/reports/trafficback`
- `GET /api/reports/health`

Supported query parameters:

- `from`: `YYYY-MM-DD` or RFC3339 timestamp.
- `to`: `YYYY-MM-DD` or RFC3339 timestamp.
- `timezone`: IANA timezone name, defaults to `UTC`.
- `campaign_id`
- `stream_id`
- `destination_id`
- `source_id`

Filter IDs accept only letters, numbers, `_` and `-`. Report group fields are selected from a fixed whitelist in code.

## ClickHouse tables

The report repository expects these event tables:

- `click_events`
- `conversion_events`
- `trafficback_events`
- `destination_health_events`

Overview and campaign/stream/destination/source reports aggregate clicks, conversions, revenue, cost, profit and ROI from click and conversion events.

Trafficback and health reports aggregate event counts by reason/status because they are operational event streams, not revenue streams.
