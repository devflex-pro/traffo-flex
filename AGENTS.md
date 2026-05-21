# AGENTS.md — Codex development rules for TraffoFlex

## MVP apps

- `apps/api-service`: admin/API backend.
- `apps/traffic-service`: traffic entrypoints, redirects, trafficback, healthchecks.
- `apps/postback-service`: incoming/outbound postbacks and conversions.
- `apps/admin-frontend`: React admin UI.

Do not implement web push subscriptions or DSP push monetization unless explicitly requested later.

## Service boundaries

### api-service

Owns:
- auth;
- users/teams;
- campaign CRUD;
- stream CRUD;
- destination CRUD;
- traffic source CRUD;
- affiliate network CRUD;
- postback template CRUD;
- reports API;
- manual healthcheck trigger API.

Frontend talks to api-service only.

api-service must not handle high-load traffic clicks.
api-service must not handle public postbacks.

### traffic-service

Owns:
- campaign endpoints;
- redirect engine;
- rule matching;
- destination selection;
- trafficback;
- destination health checks;
- click logging.

### postback-service

Owns:
- incoming postbacks;
- conversion normalization;
- deduplication;
- outbound postbacks;
- postback logs.

## Go rules

- Use Go 1.22+.
- Use `net/http` and `chi`.
- Do not use `fasthttp` in MVP.
- Use context-aware functions.
- Use structured JSON logging.
- Keep handlers thin.
- Put business logic in internal packages.
- Use explicit errors.
- Never ignore errors with `_`; every returned error must be handled, propagated, logged with context, or explicitly documented as impossible.
- If a function declaration or function call has more than one argument, format it with one argument per line, including a trailing comma where the language supports it.
- Do not trust forwarded IP headers unless remote address is a trusted proxy.
- Use `net/netip` for IP parsing.
- Keep redirect hot path minimal.

## Data rules

- MongoDB is the source of truth for config.
- ClickHouse is the source of truth for analytics events.
- Do not query MongoDB on every click.
- Active campaigns should be cached in traffic-service memory.
- ClickHouse writes should be batched or asynchronous.
- Store raw query, normalize known fields.
- Never let arbitrary inbound `url` or `redirect_url` override configured destinations.

## Frontend rules

- Use React + TypeScript + Vite.
- Use Tailwind CSS and shadcn/ui style conventions.
- Use TanStack Query for API state.
- Use TanStack Table for data grids.
- Use Recharts for charts.
- Use react-hook-form + zod for forms.
- Frontend must call api-service, not traffic-service or postback-service directly.

## Priority tests

- client IP resolver;
- macro renderer;
- rule engine;
- weighted distribution;
- trafficback loop protection;
- postback deduplication;
- health status transitions;
- API validation.

## Security

Do not add features that encourage malware delivery, phishing, deceptive redirects, unauthorized cloaking, evasion, spam, or illegal activity.
