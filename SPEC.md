# TraffoFlex MVP Specification

TraffoFlex is an open-source, self-hosted MVP for traffic routing, conversion tracking, campaign analytics, and monetization workflows. This repository is a foundation for further production product development, not a finished production distribution.

## Product Goal

TraffoFlex helps teams manage paid traffic flows in one controlled system:

- route campaign traffic through configurable streams and destinations;
- track clicks, conversions, postbacks, revenue, cost, profit, and ROI;
- keep configuration in MongoDB and analytics events in ClickHouse;
- operate the system through a React admin UI;
- validate the full local pipeline with Docker Compose and demo data.

## MVP Applications

```text
apps/
  api-service/        # Go API backend for admin UI and CRUD
  traffic-service/    # Go public traffic entrypoints and redirects
  postback-service/   # Go public postback and conversion service
  admin-frontend/     # React admin panel
```

The admin frontend talks only to `api-service`.

`traffic-service` and `postback-service` do not expose admin CRUD APIs. They own high-load public flows and internal operational endpoints.

Web push subscriptions and DSP push monetization are intentionally outside the MVP scope.

## Technology Stack

Backend:

- Go 1.22+
- `net/http`
- `chi`
- MongoDB
- ClickHouse
- Redpanda/Kafka-compatible event transport
- Docker Compose for local runtime

Frontend:

- React
- TypeScript
- Vite
- Tailwind CSS
- shadcn/ui style conventions
- TanStack Query
- TanStack Table
- Recharts
- react-hook-form
- zod

## Data Ownership

MongoDB is the source of truth for configuration and operational state:

- users and teams;
- campaigns;
- streams;
- destinations;
- traffic sources;
- affiliate networks;
- postback templates;
- health state;
- postback logs and dedupe state where needed.

ClickHouse is the source of truth for analytics events:

- click events;
- conversion events;
- postback log events;
- trafficback events;
- destination health events;
- reporting aggregates.

## Service Responsibilities

### api-service

Owns:

- auth and user approval;
- campaign CRUD;
- stream CRUD;
- destination CRUD;
- traffic source CRUD;
- affiliate network CRUD;
- postback template CRUD;
- reports API;
- postback log browsing;
- destination health history;
- Kafka ingestion error browsing;
- manual destination healthcheck trigger;
- traffic-service cache reload integration.

It must not process public traffic clicks or public postbacks.

### traffic-service

Owns:

- campaign endpoints;
- click ID generation;
- query parameter parsing and normalization;
- trusted proxy aware client IP resolution;
- request context building;
- rule matching;
- stream selection;
- destination availability checks;
- destination selection;
- redirect responses;
- trafficback routing;
- trafficback loop protection;
- destination health checks;
- asynchronous click and trafficback event publishing;
- in-memory campaign configuration cache.

The redirect hot path must not query MongoDB per click.

### postback-service

Owns:

- incoming GET and JSON POST postbacks;
- postback template matching;
- conversion normalization;
- secret/token validation;
- click lookup and enrichment;
- conversion deduplication;
- postback logs;
- conversion event publishing;
- outbound postback rendering and retry.

### admin-frontend

Owns the browser UI for:

- login and approval status;
- dashboard;
- reports;
- campaign, stream, destination, traffic source, affiliate network, and postback template management;
- destination schedules and caps;
- destination health history;
- postback logs;
- ingestion errors;
- user approval.

## Routing Flow

1. A user opens a campaign URL.
2. `traffic-service` resolves the campaign from in-memory cache.
3. The request context is built from query parameters, IP, user agent, source fields, UTM fields, sub IDs, cost, and currency.
4. Streams are evaluated by priority and conditions.
5. Unavailable destinations are filtered out.
6. Optional anti-repeat routing removes destinations already used by the same user key.
7. The selector chooses a destination by waterfall, round robin, weighted, or best ROI strategy.
8. The destination URL is rendered with safe macros.
9. The user is redirected.
10. A click event is published asynchronously and ingested into ClickHouse.

Inbound parameters such as `url` or `redirect_url` must never override configured destination URLs.

## Postback Flow

1. An affiliate network sends a postback to `postback-service`.
2. The service validates the network/template and secret or token.
3. The payload is normalized into a conversion.
4. The original click is looked up in ClickHouse when available.
5. The conversion is deduplicated by network scope and transaction ID.
6. Conversion and postback log events are published to Redpanda.
7. ClickHouse ingests the events for reports.

## Reports

Reports are backed by ClickHouse and include:

- overview metrics;
- daily time series;
- campaigns;
- streams;
- destinations;
- traffic sources;
- trafficback;
- health events;
- ingestion errors.

Core metrics:

- clicks;
- conversions;
- revenue;
- cost;
- profit;
- ROI;
- postback/log event counts.

## Production Hardening Still Required

This MVP should be hardened before production use:

- review and strengthen auth/session settings;
- move secrets to an external secret manager;
- add production backup and restore runbooks;
- add observability dashboards and alerts;
- expand load tests and integration tests;
- define deployment, upgrade, and rollback procedures;
- complete a security review and threat model.

## Responsible Use

TraffoFlex is intended for legitimate traffic routing, campaign analytics, A/B testing, affiliate tracking, and conversion attribution.

It must not be used for malware delivery, phishing, deceptive redirects, unauthorized cloaking, evasion of security systems, spam, or illegal activity.
