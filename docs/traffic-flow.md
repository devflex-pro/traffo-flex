# Traffic Flow

This document describes the full traffic path in TraffoFlex: from an incoming click to redirect, analytics, postback attribution and reports.

## Short Version

```text
User click
 -> traffic-service
 -> cached campaign config
 -> stream match
 -> destination availability
 -> unique routing / caps / schedule / health
 -> destination selection
 -> redirect
 -> Redpanda click event
 -> ClickHouse click_events
 -> affiliate postback
 -> postback-service
 -> ClickHouse click lookup
 -> dedupe
 -> Redpanda conversion event
 -> ClickHouse conversion_events
 -> api-service reports
 -> admin frontend
```

## 1. Incoming Click

A user opens a campaign URL, for example:

```http
GET /c/{campaignSlug}?sub1=...&utm_source=...&cost=...
```

The request is handled by `traffic-service`.

Supported public traffic endpoints:

- `GET /c/{campaignSlug}`
- `GET /go/{campaignPublicId}`
- `GET /r/{publicToken}`
- `GET /tb/{campaignSlug}`

## 2. Request Context

`traffic-service` builds a request context from the incoming request.

It extracts:

- generated click ID;
- campaign slug, public ID or public token;
- IP address;
- user agent;
- raw query params;
- `sub1` through `sub10`;
- UTM params;
- cost and currency;
- source click ID.

Forwarded IP headers are trusted only when the request comes from a configured trusted proxy.

## 3. Campaign Config Cache

Active campaign config is read from the in-memory cache.

MongoDB is not queried on every click. `traffic-service` loads config from MongoDB on startup and through internal cache reload.

The cached config includes:

- campaigns;
- streams;
- destinations;
- destination policies;
- stream distribution settings.

## 4. Stream Selection

Inside the selected campaign, streams are evaluated by priority and conditions.

The first matching stream becomes the active stream for the click.

If no stream matches, traffic falls back to the campaign trafficback flow where configured.

Stream conditions can match normalized request fields:

- source: `source_id`, `source_click_id`;
- geo: `geo_country`, `geo_region`, `city`, `asn`, `isp`;
- device: `device_type`, `os`, `browser`;
- request: `ip`, `user_agent`, `referrer`;
- tracking: `sub1` through `sub10`, UTM params, `cost`, `currency`;
- raw query params through both `<param>` and `query.<param>`.

Common query aliases are normalized. For example, `country=US` fills `geo_country`, `device=mobile` fills `device_type`, and `platform=Android` fills `os`.

## 5. Destination Availability

Before choosing a destination, `traffic-service` removes unavailable destinations.

A destination is unavailable when:

- `manual_status` is not `active`;
- `health_status` is `unhealthy`;
- the current time is outside the configured schedule;
- a cap is reached;
- unique routing excludes this destination for the current user key.

Caps are checked against ClickHouse and use a short TTL cache.

Supported cap metrics:

- `clicks`;
- `cost`;
- `conversions`;
- `revenue`.

Cap windows are configured in hours, so hourly, daily and weekly limits are represented as `1`, `24` and `168`.

## 6. Unique Destination Routing

If stream `distribution.unique_policy.enabled=true`, TraffoFlex tries not to send the same user key to the same destination again while other suitable destinations are available.

Supported user keys:

- `source_click_id`;
- `user_agent`;
- `sub1` through `sub10`;
- `utm_source`, `utm_medium`, `utm_campaign`, `utm_content`, `utm_term`;
- `query.<param>`.

The flow is:

1. Read used destinations for this user key from ClickHouse `click_events`.
2. Exclude destinations already used inside `history_window_hours`.
3. If every destination was already used, apply `exhausted_mode`.

Supported exhausted modes:

- `allow_repeat`: allow repeat and choose again from the available list.
- `no_destination`: leave no destination and use the normal fallback path.

## 7. Destination Selection

After availability filtering, selector chooses the final destination.

Supported strategies:

- `waterfall`: choose the first available destination in configured order.
- `round_robin`: rotate available destinations for the current click.
- `weighted`: distribute traffic by configured weights.
- `best_roi`: rank destinations by ROI from ClickHouse analytics.

For `best_roi`, the router uses:

- `roi_window_hours`;
- `min_clicks`;
- `fallback_strategy`.

If there is not enough ROI data, the selector uses `fallback_strategy`, usually `round_robin`.

## 8. Redirect URL Rendering

The selected destination URL is rendered with safe macros.

Common macros include:

- `{click_id}`;
- `{sub1}`;
- `{sub2}`;
- `{utm_source}`;
- `{utm_campaign}`.

Inbound query params like `url` or `redirect_url` cannot override configured destination URLs.

## 9. User Redirect

`traffic-service` returns the configured redirect response, usually HTTP 302, to send the user to the selected destination.

The redirect hot path keeps MongoDB out of per-click execution.

## 10. Click Event Publishing

After destination selection, `traffic-service` publishes a click event to Redpanda:

```text
traffic-service -> traffoflex.click_events
```

Click logging is asynchronous. Temporary analytics write failures do not block the redirect.

## 11. ClickHouse Ingestion

ClickHouse consumes Redpanda topics through Kafka engine tables and materialized views.

Click events become available in:

```text
click_events
```

These records are later used for:

- reports;
- destination caps;
- unique routing history;
- postback click lookup;
- ROI ranking.

## 12. Affiliate Postback

When a conversion happens, the affiliate network calls `postback-service`.

Examples:

```http
GET /pb/{network}?click_id=...&payout=...&txid=...
```

or JSON POST:

```http
POST /api/postbacks
```

## 13. Postback Normalization

`postback-service` applies the configured postback template and normalizes the incoming payload into a conversion.

It handles:

- secret or token validation;
- field mapping;
- transaction ID extraction;
- payout/revenue parsing;
- status parsing;
- raw payload storage in logs.

## 14. Click Lookup and Enrichment

`postback-service` looks up the original click in ClickHouse by click ID.

The conversion is enriched with:

- campaign ID;
- stream ID;
- destination ID;
- source data;
- click metadata.

This links revenue back to the original traffic routing decision.

## 15. Conversion Dedupe

Before storing and publishing the conversion, `postback-service` deduplicates it by network scope and transaction ID.

Duplicate transaction IDs do not create duplicate conversions.

## 16. Conversion Event Publishing

The normalized conversion is published to Redpanda:

```text
postback-service -> traffoflex.conversion_events
```

ClickHouse consumes it into:

```text
conversion_events
```

## 17. Reports

`api-service` reads reports from ClickHouse.

Reports include:

- clicks;
- conversions;
- revenue;
- cost;
- profit;
- ROI;
- grouped reports by campaign;
- grouped reports by stream;
- grouped reports by destination;
- grouped reports by source;
- trafficback reports;
- health reports.

`admin-frontend` talks only to `api-service`.

## Operational Notes

- MongoDB is the source of truth for configuration and operational state.
- ClickHouse is the source of truth for analytics events.
- Redpanda is the event transport between services and ClickHouse.
- `api-service` does not handle public clicks.
- `postback-service` owns public postbacks.
- `traffic-service` owns the redirect hot path.
- Full local verification is available through `make e2e-smoke`.
