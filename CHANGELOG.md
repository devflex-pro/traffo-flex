# Changelog

Notable changes are recorded here. Entries under `Unreleased` have not been
published. Before pushing a release tag, move them to a dated `vX.Y.Z` section.

## [Unreleased]

## [v0.1.4] - 2026-10-08

### Changed

- Make the incoming postback form universal across affiliate networks, remove
  provider presets and the destination smartlink builder, and expose optional
  transaction/status macros and payout currency. Existing callback URLs remain
  valid.
- Simplify the incoming log to an ID search, UTC period filters and a sortable
  time column, with server-side search, sorting and pagination across workspace
  records.

## [v0.1.3] - 2026-10-08

### Added

- Separate incoming and outgoing postback forms, a LosPollos preset with
  copyable callback and smartlink builders, and an outgoing delivery log.
- Persistent outbound callback delivery with owner/source/campaign scopes,
  original source click IDs, query-encoded macros, bounded retries and recovery
  after restart. Historical conversions are not replayed automatically.
- A public postback origin in client configuration and production Compose.

## [v0.1.2] - 2026-10-04

### Added

- A campaign tracking URL builder with copyable links, configurable query
  parameters and traffic source macros. Saved parameters are reused by the
  campaign list copy action; existing campaigns retain the default template.
- A dedicated streams page for each campaign, stream and unique destination
  counts in the campaign list, and direct links to campaign reports.
- Campaign archive and restore actions, with permanent deletion available only
  from the archive after confirmation.
- Status icons and labeled action buttons throughout the admin UI, plus a
  colored destination health indicator that updates after a manual probe.

### Fixed

- Allow manual destination probes before a campaign is active and require a
  trafficback destination when activating a campaign.
- Use the correct healthcheck address for the local admin frontend container.
- Provide a tracker domain to the production Compose validation step in CI.

## [v0.1.1] - 2026-10-04

### Fixed

- Show masked feedback when entering the Resend API key during installation.
- Clarify that the sender address for login codes must use a Resend-verified domain.
- Restore the original file-creation mask after saving secrets and make the
  ACME webroot readable by Nginx, including on installer retries.
- Keep Docker Compose child processes from consuming the remaining installer
  script when the one-command installation runs through `curl | bash`.

## [v0.1.0] - 2026-10-04

### Added

- A production single-node Docker Compose stack, with a separate reusable
  Nginx and Certbot stack, persistent data volumes and no public database ports.
- Replicated ClickHouse local tables and Distributed public tables, starting
  with one shard and one replica; minute and hour click rollups.
- A bounded, persistent click WAL with background Redpanda delivery and
  duplicate-event auditing.
- Persistent routing snapshots, background destination availability snapshots,
  and Bloom-filter anti-repeat checks outside the database-dependent click path.
- Configured trafficback routing when destinations are unavailable, with
  destination probe failure and recovery thresholds.
- Background delivery and attribution of accepted postbacks, with retry state
  stored in MongoDB.
- An admin action to refresh the traffic routing snapshot after configuration
  changes.
- Release CI that runs checks on PRs and `main`, publishes `v*` and `latest`
  GHCR images from release tags on `main`, and verifies anonymous image access.
- A versioned GHCR deployment bundle, a README copy-paste command that resolves
  and pins the latest release, and an English Debian 13 deployment guide.
- Installer prompts and a validated Nginx template for custom tracker, postback
  and admin domains, shared by Certbot, the proxy and the API origin.
- Per-user configuration and analytics isolation, an administrator workspace
  switcher with full edit access, and a two-user end-to-end isolation check.
- HttpOnly session cookies, CSRF checks for browser mutations, OTP attempt
  limits and session revocation on logout.

### Changed

- The production tracker starts without waiting for MongoDB, ClickHouse or
  Redpanda. With no usable routing snapshot it stays unready and retries MongoDB
  instead of exiting; an existing snapshot keeps redirects available.
- Accepted postbacks are acknowledged after durable MongoDB persistence;
  Redpanda delivery and ClickHouse attribution run in the background.
- Reports use minute and hour rollups; second-level reporting precision is no
  longer required.
