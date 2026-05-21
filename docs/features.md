# TraffoFlex Features

This document describes the current MVP feature set.

## Current Status

TraffoFlex is implemented as a self-hosted MVP for traffic routing, conversion tracking, analytics, and campaign administration. The main `click -> Redpanda -> ClickHouse -> reports` flow and the `postback -> conversion enrichment -> Redpanda -> ClickHouse -> reports` flow can be validated locally with Docker Compose and `make e2e-smoke`.

The project contains four applications:

- `apps/api-service`: admin/API backend.
- `apps/traffic-service`: public traffic endpoints, redirects, trafficback, and health checks.
- `apps/postback-service`: incoming postbacks, conversion normalization, dedupe, and outbound postback retry.
- `apps/admin-frontend`: React admin UI.

The frontend calls `api-service` only.

## Runtime Stack

The local self-hosted stack is defined in `deploy/docker-compose.yml` and includes:

- MongoDB for configuration, users, and operational state;
- a three-broker Redpanda cluster;
- Redpanda Console;
- ClickHouse for analytics events and reports;
- `api-service`;
- `traffic-service`;
- `postback-service`;
- `admin-frontend`.

Redpanda topics:

- `traffoflex.click_events`
- `traffoflex.conversion_events`
- `traffoflex.postback_log_events`
- `traffoflex.trafficback_events`
- `traffoflex.destination_health_events`

ClickHouse reads these topics through Kafka engine tables and materialized views.

## Auth And Users

`api-service` supports:

- email OTP authentication;
- JWT sessions;
- OTP TTL;
- OTP send rate limiting;
- resend prevention while a valid OTP exists;
- Resend delivery integration;
- local/dev OTP return mode;
- admin user configured by environment;
- automatic approval for the admin email;
- approval flow for other users;
- `pending_approval` state after email confirmation;
- admin user approval endpoint;
- `/api/me`;
- paginated users list.

## Admin API

`api-service` provides CRUD and admin endpoints for:

- campaigns;
- streams;
- destinations;
- traffic sources;
- affiliate networks;
- postback templates;
- manual destination healthcheck trigger;
- traffic-service cache reload integration;
- postback-service internal actions where needed;
- reports API;
- postback logs;
- destination health history;
- Kafka ingestion errors.

List endpoints use a shared response contract:

```json
{
  "items": [],
  "limit": 100,
  "offset": 0,
  "total": 0
}
```

## Traffic Routing

`traffic-service` exposes public endpoints:

- `GET /c/{campaignSlug}`
- `GET /go/{campaignPublicId}`
- `GET /r/{publicToken}`
- `GET /tb/{campaignSlug}`

Implemented routing capabilities:

- active campaign configuration loaded from MongoDB on startup;
- in-memory cache for the redirect hot path;
- no MongoDB query per click;
- internal cache reload endpoint;
- request context builder;
- click ID generation;
- query parameter parsing;
- sub IDs, cost, and currency capture;
- trusted-proxy-aware IP resolver;
- safe forwarded header handling;
- stream rule matching by source, geo, device, browser, user agent, UTM, sub IDs, and raw query parameters;
- weighted destination distribution;
- destination health awareness;
- destination schedule windows by weekday, time, and timezone;
- destination caps by clicks, cost, conversions, and revenue with custom hour windows;
- anti-repeat destination routing by configurable user key;
- destination selection by waterfall, round robin, or best ROI with fallback;
- trafficback routing;
- trafficback loop protection;
- safe macro rendering in destination URLs;
- protection against inbound `url` and `redirect_url` overrides;
- click event publishing to Redpanda;
- trafficback event publishing to Redpanda.

## Destination Healthchecks

Implemented healthcheck capabilities:

- manual healthcheck trigger through `api-service`;
- internal trigger endpoint in `traffic-service`;
- periodic destination healthcheck worker;
- Mongo-backed current destination health state;
- health transition history;
- destination health event publishing to Redpanda;
- admin UI for destination health history.

## Destination Schedules And Caps

Destinations can be available only during configured weekday/time windows in a timezone. This supports business schedules such as Monday-Friday 09:00-18:00 and overnight windows such as Monday 22:00-Tuesday 02:00.

Destinations can also stop receiving traffic when cap rules are reached:

- `clicks`: maximum clicks in a custom hour window.
- `cost`: maximum click cost in a custom hour window.
- `conversions`: maximum conversions in a custom hour window.
- `revenue`: maximum postback payout or revenue in a custom hour window.

Examples:

- hourly cap: `window_hours=1`;
- daily cap: `window_hours=24`;
- weekly cap: `window_hours=168`;
- custom cap: any positive hour window, for example `6`, `12`, or `72`.

Caps are checked by `traffic-service` against ClickHouse with a short TTL cache. If a destination reaches a cap, it is treated as unavailable and the selector tries the next destination or trafficback flow.

## Anti-Repeat Destination Routing

Streams can enable `unique_policy` to avoid sending the same user key to the same destination again while other matching destinations are still available.

Supported user keys:

- `source_click_id`;
- `user_agent`;
- `sub1` through `sub10`;
- `utm_source`, `utm_medium`, `utm_campaign`, `utm_content`, `utm_term`;
- `query.<param>` for a raw query parameter.

Selection strategies:

- `waterfall`: first available destination in configured order.
- `round_robin`: deterministic rotation over available destinations for the current click.
- `best_roi`: rank available destinations by ClickHouse ROI over a configured window.

When `best_roi` has too little data, TraffoFlex uses the configured fallback strategy, usually `round_robin`. If every destination was already used by the same user key inside `history_window_hours`, `exhausted_mode=allow_repeat` lets the selector use the available list again, while `no_destination` leaves no destination and sends traffic to the normal fallback path.

## Postbacks And Conversions

`postback-service` supports:

- `GET /pb/{network}`;
- `POST /api/postbacks`;
- template-based normalization;
- GET and JSON POST postbacks;
- field mapping through postback templates;
- secret/token validation;
- conversion model;
- click lookup enrichment from ClickHouse;
- configurable click lookup fail-open/fail-closed behavior;
- dedupe by network scope and transaction ID;
- MongoDB persistence for conversions/dedupe;
- MongoDB postback logs;
- postback log event publishing to Redpanda;
- conversion event publishing to Redpanda;
- outbound postback rendering;
- bounded outbound retry worker;
- internal outbound retry endpoint;
- internal test postback endpoint.

## Analytics And Reports

`api-service` reads analytics from ClickHouse and exposes:

- overview metrics;
- daily report series;
- grouped reports by campaigns;
- grouped reports by streams;
- grouped reports by destinations;
- grouped reports by sources;
- grouped reports by trafficback;
- grouped reports by health;
- ingestion errors report.

Supported report behavior:

- time range filters;
- timezone validation;
- safe whitelist for report group fields;
- flexible numeric decoding for ClickHouse `JSONEachRow` responses.

## Admin Frontend

`apps/admin-frontend` is built with React, TypeScript, and Vite.

The UI includes:

- auth shell;
- OTP login flow;
- protected routes;
- pending approval state;
- dashboard;
- reports tables and charts;
- campaign CRUD;
- stream management;
- destination CRUD;
- stream rule editor;
- stream distribution editor with destination targets, weights, waterfall, round robin, best ROI, and unique routing policy;
- destination schedule editor;
- destination caps editor;
- manual healthcheck action;
- postback template management;
- filtered postback logs;
- destination health history;
- Kafka ingestion errors;
- users list and admin approve action;
- typed API client;
- TanStack Query integration;
- TanStack Table grids;
- Recharts charts;
- react-hook-form and zod forms.

## Observability And Operations

Implemented:

- `/healthz` for process liveness;
- `/readyz` for dependency/config readiness;
- structured JSON logs;
- request ID middleware;
- CORS config for the admin frontend;
- Docker Compose startup ordering;
- `make e2e-smoke` for a full local smoke test;
- `GET /internal/event-producer/stats` in `traffic-service` and `postback-service`.

Event producer stats include:

- topic;
- queue capacity;
- queued events;
- published count;
- dropped count;
- publish errors;
- failed enqueue count;
- closed state.

## Security And Safety

Implemented guardrails:

- admin UI does not call public services directly;
- `api-service` does not accept public clicks;
- `api-service` does not accept public postbacks;
- redirect targets are taken only from configured destinations;
- inbound `url` and `redirect_url` cannot override destinations;
- forwarded IP headers are trusted only with trusted proxy configuration;
- validation for slugs, statuses, URLs, macro names, currencies, and report filters;
- JWT session secret is required outside unsafe local defaults;
- explicit error handling in Go code.
