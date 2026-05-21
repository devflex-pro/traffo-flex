# TraffoFlex implementation plan

## Current state

Project is a verified MVP aligned with `SPEC.md` service boundaries. The core backend, analytics pipeline, admin UI and local/self-hosted runtime wiring are implemented, and the main click/postback/report paths have been checked against a live local stack.

- `api-service` exposes health/readiness, email OTP auth with JWT sessions and admin approval, admin CRUD for campaigns, streams, destinations, traffic sources, affiliate networks and postback templates, integration trigger endpoints and ClickHouse-backed reports. Runtime admin/config repositories are Mongo-backed in application startup.
- `traffic-service` exposes cached redirect endpoints for campaign slug, campaign public ID and public token routing, with request context building, rule matching, weighted destination selection, anti-repeat destination routing, best ROI ordering, trafficback protection, destination health awareness, destination schedules/caps and async click logging to Redpanda. Startup cache loading is Mongo-backed while the redirect hot path stays in memory.
- `postback-service` accepts GET/POST postbacks, validates simple secrets/tokens, normalizes payloads, deduplicates conversions, enriches conversions from ClickHouse click events, persists conversions/dedupe/postback logs in MongoDB, emits conversion and postback log events to Redpanda and supports outbound postback queue/retry.
- `admin-frontend` provides an auth shell, typed API client, dashboard, reports, campaign/stream/destination CRUD including schedule/cap JSON policy fields, postback template management, filtered postback logs, destination health history and Kafka ingestion error browsing.
- `packages/go-shared` contains shared IDs, logging, env helpers, HTTP helpers, macro rendering, client IP resolving, domain models and validation helpers.
- `deploy/docker-compose.yml` starts Redpanda, MongoDB, ClickHouse and all apps.
- Go unit/router tests exist for the implemented backend behavior.
- Current readiness estimate: 90%+ for MVP. Remaining work is production hardening, not core MVP behavior.

## Guiding rules

- Keep the four-service MVP boundary strict: admin UI talks only to `api-service`; public traffic and public postbacks stay out of `api-service`.
- Prefer small reversible changes with tests close to the changed behavior.
- Do not add new dependencies unless they remove clear implementation risk or are already required by the chosen stack.
- MongoDB is the source of truth for configuration and operational state.
- ClickHouse is the source of truth for analytics events.
- Do not query MongoDB on every click; traffic runtime config must be cached in memory.
- Never allow inbound `url` or `redirect_url` parameters to override configured destinations.
- Security-sensitive behavior must fail closed.

## Phase 0: baseline hardening

Goal: make the skeleton predictable before adding domain behavior.

Status: done.

- Add minimal request/response helpers for JSON encoding, validation errors and method-safe error responses.
- Add structured logging middleware with request ID, path, status and latency.
- Add panic recovery middleware.
- Add CORS configuration for local admin frontend development.
- Add context-aware config loading for MongoDB, ClickHouse and service-to-service URLs.
- Add health/readiness checks that distinguish process liveness from dependency readiness.
- Add smoke tests for routers and health endpoints.

Done when:

- `make test` passes.
- Each Go app has router tests for `/healthz` and `/readyz`.
- Readiness can report unavailable dependencies without breaking `/healthz`.

## Phase 1: shared domain contracts

Goal: define stable data structures before persistence and UI work.

Status: done.

- Expand shared models for campaigns, streams, destinations, traffic sources, affiliate networks, postback templates and health statuses.
- Define redirect request context fields: click ID, campaign, source, IP, user agent, query params, sub IDs, cost and currency.
- Define analytics event payloads for clicks, conversions, postback logs, trafficback events and health events.
- Define validation rules for slugs, public IDs, status values, weights, URLs and macro names.
- Add focused tests for validation, ID generation assumptions, macro rendering and client IP resolution.

Done when:

- Core DTOs and domain structs are documented in code.
- Validation rejects unsafe redirect URL overrides and invalid configured destination URLs.
- Priority shared-package tests exist and pass.

## Phase 2: api-service CRUD and auth foundation

Goal: make `api-service` the real admin/config owner.

Status: done for Mongo-backed MVP repositories and email OTP/JWT auth. Pagination/filtering consistency remains production follow-up work.

- Add MongoDB connection lifecycle and repository interfaces.
- Implement email OTP auth, JWT sessions, env-defined admin user and admin approval for other users.
- Implement CRUD for campaigns, streams, destinations, traffic sources, affiliate networks and postback templates.
- Add API validation with explicit error responses.
- Add pagination/filtering basics for list endpoints.
- Add internal client calls for traffic cache reload and manual destination healthcheck trigger.
- Add handler and repository tests around validation and persistence behavior.

Done when:

- Admin CRUD endpoints from `SPEC.md` are implemented or explicitly marked out of scope.
- Frontend can manage campaigns, streams and destinations through `api-service`.
- Invalid data is rejected before reaching MongoDB.

## Phase 3: traffic-service redirect engine

Goal: implement the hot path without per-click MongoDB reads.

Status: done for the MVP Mongo-loaded, in-memory hot path cache contract, Mongo-backed destination health state, destination schedules/caps, anti-repeat destination routing, best ROI ordering, real manual/periodic destination probes and Redpanda-backed click, trafficback and destination health event publishing.

- Add campaign config cache with startup load, periodic refresh and `/internal/cache/reload`.
- Implement request context builder with trusted-proxy IP resolving.
- Implement rule matching for stream selection.
- Implement weighted destination distribution.
- Implement trafficback routing and loop protection.
- Implement destination health awareness in selection and Mongo-backed probe-triggered status updates.
- Implement destination schedule and cap availability policies.
- Implement anti-repeat destination routing by user key with waterfall, round robin and best ROI strategies.
- Render destination macros safely.
- Write click, trafficback and destination health events to Redpanda; ClickHouse consumes the topics through Kafka engine tables and materialized views.
- Add tests for IP resolver, rule engine, distribution, macro rendering and trafficback loop protection.

Done when:

- `/c/{campaignSlug}`, `/go/{campaignPublicId}` and `/r/{publicToken}` route using cached config.
- MongoDB is not queried in the redirect hot path.
- Configured destination URLs are the only redirect targets.
- Schedule/cap checks can make a destination unavailable before weighted selection.
- Unique policy can exclude destinations already used by the same user key and rank remaining destinations by waterfall, round robin or best ROI.
- Click logging failures do not block redirects unless explicitly configured to fail closed.

## Phase 4: postback-service conversion flow

Goal: normalize and store conversions reliably.

Status: done for MVP Mongo-backed conversion/dedupe/postback log repositories, Redpanda-backed conversion/postback event publishing and ClickHouse-backed click lookup enrichment.

- Add incoming postback template loading and validation.
- Normalize GET and POST postbacks into a conversion payload.
- Validate network tokens/signatures/secrets.
- Add click lookup contract against ClickHouse or a dedicated lookup store.
- Implement deduplication by transaction ID and network scope.
- Persist conversion state/dedupe state in MongoDB.
- Write conversion and postback log events to Redpanda; ClickHouse consumes the topics through Kafka engine tables and materialized views.
- Implement outbound postback templates and retry worker.
- Add tests for normalization, token validation, dedupe and outbound rendering.

Done when:

- Duplicate transaction IDs do not create duplicate conversions.
- Invalid or unsigned postbacks are rejected and logged.
- Outbound postbacks are retried with bounded attempts and observable status.

## Phase 5: reports and analytics

Goal: expose useful MVP metrics from ClickHouse.

Status: done for the MVP ClickHouse HTTP query adapter, report API contract and frontend report consumption.

- Implement reports overview from ClickHouse.
- Add reports grouped by campaign, stream, destination, source, trafficback and health status.
- Add time range, timezone and basic filter parameters.
- Add safe query builders or parameterized query paths.
- Add API tests with a ClickHouse abstraction or test repository.

Done when:

- Dashboard values come from analytics events instead of demo constants.
- Reports APIs match frontend table/chart needs.
- Query filters are validated and cannot produce unsafe SQL.

## Phase 6: admin frontend MVP

Goal: make the admin UI usable for the implemented API surface.

Status: done for the currently implemented API surface. The UI has auth shell, typed API client, dashboard, reports, campaign/stream/destination CRUD, postback template management, filtered postback log browsing, destination health history browsing and Kafka ingestion error browsing.

- Add API client layer with typed responses and error handling.
- Add auth flow and protected routes.
- Build CRUD screens for campaigns, streams and destinations.
- Add destination health actions and status display.
- Add postback template/log screens.
- Add dashboard and reports using TanStack Query, TanStack Table and Recharts.
- Add form validation with react-hook-form and zod.
- Add build verification and critical UI smoke checks.

Done when:

- `npm run build` passes in `apps/admin-frontend`.
- Main admin workflows work through `api-service` only.
- UI does not call `traffic-service` or `postback-service` directly.

## Phase 7: operational readiness

Goal: make local and self-hosted operation reliable.

Status: done for MVP local/self-hosted readiness with Redpanda plus ClickHouse Kafka ingestion. Remaining production work is real external secret management, stronger observability and production backup tooling.

- Add `.env.example` for all required settings.
- Add Docker healthchecks.
- Add service startup ordering notes and readiness behavior.
- Add indexes for MongoDB collections.
- Add ClickHouse schema migrations for all MVP events and Kafka ingestion views.
- Add backup/restore notes for MongoDB and ClickHouse.
- Add basic observability docs for logs, counters and failed postbacks.

Done when:

- Fresh checkout can run through documented quick start.
- Missing environment variables fail with clear messages.
- Data stores have required indexes and schemas.

## Completed Execution

1. Phase 0: baseline hardening and tests - done.
2. Phase 1: shared contracts and validation - done.
3. Phase 2: `api-service` CRUD and auth foundation - MVP done with email OTP/JWT auth.
4. Phase 3: redirect engine using cached campaign config - done.
5. Phase 4: postback normalization, dedupe and click enrichment - done.
6. Phase 5: analytics reports from ClickHouse - done.
7. Phase 6: frontend workflows on top of stable API - done for implemented API surface.
8. Phase 7: operational polish and docs - MVP done.

## E2E Verification

Goal: prove the implemented pieces work together in a live local stack.

Status: done on 2026-05-16 against Docker Compose with Redpanda, MongoDB, ClickHouse and all application services.

- Started the full Docker Compose stack from `.env.example`.
- Verified MongoDB, Redpanda, ClickHouse and backend service healthchecks.
- Verified `api-service`, `traffic-service`, `postback-service` and `admin-frontend` HTTP readiness.
- Executed a click flow through `traffic-service` and verified the click event in ClickHouse.
- Executed a postback flow through `postback-service` and verified conversion enrichment from the click event.
- Verified `api-service` reports reflect click/conversion metrics.
- Verified Redpanda topics and ClickHouse `kafka_ingestion_errors`.
- Stopped the stack after verification.

Done when:

- Full click -> Redpanda -> ClickHouse -> reports path is verified.
- Full postback -> conversion enrichment -> Redpanda -> ClickHouse -> reports path is verified.
- No ingestion errors are present for the tested events.
- Runtime gaps found during verification were fixed: ClickHouse HTTP credentials for inter-service access and flexible numeric decoding for ClickHouse JSONEachRow report responses.
- Repeatable smoke tooling is available through `make e2e-smoke`.
- List endpoints use a shared paginated response contract with `items`, `limit`, `offset` and `total` for admin CRUD lists, users, postback logs and destination health history.
- Event producers expose in-process write/retry/failure/bytes counters through internal stats endpoints and include cumulative counters in structured write logs.
- Destinations support schedule windows by weekday/time/timezone and caps by clicks, cost, conversions and revenue with custom hour windows.
- Streams support anti-repeat destination routing by configurable user key, with ClickHouse-backed history checks and best ROI ranking.

## Data and migration risks

- MongoDB schema changes need versioned indexes and backward-compatible reads where possible.
- ClickHouse table changes should prefer additive columns; destructive changes require explicit migration and retention plan.
- Dedupe state is business-critical: changing keys after production traffic may duplicate or suppress conversions.
- Campaign cache changes can affect live routing; reload behavior should be tested before enabling automatic refresh.
- Trafficback loop protection must be validated before production use to avoid redirect loops.

## Test priorities

- Client IP resolver with trusted and untrusted proxies.
- Macro renderer with missing, escaped and unsafe values.
- Rule engine matching order and fallback behavior.
- Weighted distribution determinism/statistical sanity.
- Trafficback loop protection.
- Postback deduplication by transaction ID.
- Health status transitions.
- API validation for all admin writes.

## Production Follow-up Backlog

- Add richer Resend email templates and bounce observability for OTP messages.
- Add real external secret management for production deployments.
- Automate backup/restore tooling for MongoDB and ClickHouse.
- Promote `make e2e-smoke` into CI once CI runners can run Docker Compose.
