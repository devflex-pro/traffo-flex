# Changelog

Notable changes are recorded here. Entries under `Unreleased` have not been
published. Before pushing a release tag, move them to a dated `vX.Y.Z` section.

## [Unreleased]

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
