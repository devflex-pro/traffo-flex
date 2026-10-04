# Production deployment: first single-node release

This guide installs TraffoFlex on one Debian 13 x86_64 VPS without cloning the
repository or building images on the server. Run the commands as root. The host
needs Docker Engine, the Docker Compose plugin, systemd and standard utilities.
Only shared Nginx publishes ports 80 and 443; data stores stay private.

Pushes and PRs to `main` run tests without publishing images. A pushed `v*` tag
whose commit belongs to `main` publishes five application images and one
`deploy-bundle` image to GHCR. All six get the release tag; after all six builds
succeed, CI promotes them to `latest`. The installer resolves `latest` to that
release tag and pins all application images to it.

## One-command installation

Before running the installer, choose three distinct lowercase DNS names for
the tracker, postbacks and admin panel. Point their DNS A records to the VPS
and open ports 80 and 443. Docker Engine and the Compose plugin must already
be installed.
Run this from an interactive terminal:

```sh
bash -o pipefail -c 'curl -fsSL https://raw.githubusercontent.com/devflex-pro/traffo-flex/main/scripts/install-prod.sh | sudo bash -s -- --version latest'
```

If already logged in as root, omit `sudo` in the command. To pin the installer
itself to a release, replace `main` in the URL with a published `vX.Y.Z` tag
and replace `--version latest` with `--version vX.Y.Z`.

The script downloads public GHCR images, asks for the three DNS names, admin
email, a verified sender email and a Resend API key, generates database and JWT
secrets, installs shared Nginx and Certbot, issues one certificate for all three
names, starts TraffoFlex
and enables certificate renewal. It never changes the firewall or SSH settings.
The admin email may be a Gmail address. The sender email must use a domain
verified in Resend, or login codes cannot be delivered.
It preserves secrets, certificates and volumes when re-run with the same tag;
it refuses an automatic upgrade to a different tag. Re-runs use the domains
saved in `/opt/traffoflex/.env.production`. Changing domains after certificate
issuance needs a reviewed certificate and Nginx update; the installer stops if
the saved domains differ from the active Nginx configuration.

GHCR packages must be public for this command to work without a token. GitHub
initially creates container packages as private even for public repositories.
After the first release, set all six `traffo-flex-*` packages to **Public** in
the organization's GitHub Packages settings. The CI release job checks that
all six images can be read anonymously. See [GitHub's package visibility
guide](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility).

The installer stores versioned files under `/opt/traffoflex/releases/vX.Y.Z`,
the active version at `/opt/traffoflex/current`, credentials in
`/opt/traffoflex/.env.production`, and shared proxy/certificates in `/opt/infra`.
Read `/opt/traffoflex/current/README.md` for operations and release limits.
Release changes are included in `/opt/traffoflex/current/CHANGELOG.md`. Before
tagging a release, move the `Unreleased` entries into a section headed
`## [vX.Y.Z] - YYYY-MM-DD`; CI requires a section matching the tag.

## Manual installation

The following commands provide the same initial setup if you need to perform
each step manually. Do not run them after the one-command installer.

### 1. Download the release files

Choose a published release tag. The bundle contains the production Compose
file, required MongoDB init scripts, shared infra example, env template and
this guide. It contains no application source or credentials.

Public packages can be pulled without logging in. Replace `v1.0.0` with the
release you want to install:

```sh
RELEASE=v1.0.0
BUNDLE_IMAGE="ghcr.io/devflex-pro/traffo-flex-deploy-bundle:${RELEASE}"
mkdir -p "/opt/traffoflex/releases/${RELEASE}" /opt/infra /opt/backups/traffoflex
docker pull "$BUNDLE_IMAGE"
bundle_container=$(docker create "$BUNDLE_IMAGE")
docker cp "$bundle_container:/bundle/." "/opt/traffoflex/releases/${RELEASE}/"
docker rm "$bundle_container"
ln -s "releases/${RELEASE}" /opt/traffoflex/current
cat /opt/traffoflex/current/README.md
```

The server has no Git checkout. For an upgrade, extract a new bundle into a
staging directory and review its Compose and infra changes before switching
`current`. Keep `.env.production`, `/opt/infra/.env`, certificates and Docker
volumes outside the bundle.

### 2. Configure domains, start the shared proxy and issue certificates

Choose an unused subnet. The example reserves `172.30.240.10` for Nginx, so
TraffoFlex trusts exactly `172.30.240.10/32` for forwarded client IPs. If the
subnet conflicts with another Docker network, change both `/opt/infra/.env`
and `NGINX_PROXY_CIDR` in the app env file.

Create `/opt/traffoflex/.env.production` now. Set `IMAGE_TAG` to the selected
release, enter three distinct lowercase DNS names in `TRACKER_DOMAIN`,
`POSTBACK_DOMAIN` and `APP_DOMAIN`, and fill all credential values. MongoDB and
ClickHouse passwords must be URL-safe alphanumeric values because their
connection URLs contain those values. Point the three names to the VPS before
requesting a certificate.

```sh
cp /opt/traffoflex/current/.env.production.example /opt/traffoflex/.env.production
chmod 600 /opt/traffoflex/.env.production
# Edit /opt/traffoflex/.env.production and fill IMAGE_TAG, domains and credentials.
```

```sh
docker network inspect proxy >/dev/null 2>&1 || \
  docker network create --driver bridge --subnet 172.30.240.0/24 proxy
cp -a /opt/traffoflex/current/deploy/infra/. /opt/infra/
cd /opt/infra
cp .env.example .env
mkdir -p letsencrypt certbot-www
docker compose up -d nginx
```

Port 80 must be reachable. The initial HTTP configuration serves the ACME
webroot without requiring a certificate. Read the chosen names from the env
file and validate them before requesting the certificate:

```sh
env_file=/opt/traffoflex/.env.production
tracker_domain=$(awk -F= '$1 == "TRACKER_DOMAIN" { print $2 }' "$env_file")
postback_domain=$(awk -F= '$1 == "POSTBACK_DOMAIN" { print $2 }' "$env_file")
app_domain=$(awk -F= '$1 == "APP_DOMAIN" { print $2 }' "$env_file")
operator_email=$(awk -F= '$1 == "AUTH_ADMIN_EMAIL" { print $2 }' "$env_file")
bash /opt/traffoflex/current/render-nginx-prod.sh --validate "$tracker_domain" "$postback_domain" "$app_domain"
cd /opt/infra
docker compose run --rm certbot certonly --webroot -w /var/www/certbot \
  --cert-name traffoflex --email "$operator_email" --agree-tos --no-eff-email \
  -d "$tracker_domain" -d "$postback_domain" -d "$app_domain"
```

The `proxy` network and infra Compose are shared with future projects. Each
project can add its own Nginx vhost and certificate without joining the
TraffoFlex private network.

### 3. Start TraffoFlex

Use the published release tag already set in the production env file for all
five application images:

```sh
cd /opt/traffoflex/current
docker compose --env-file /opt/traffoflex/.env.production -f deploy/docker-compose.prod.yml config --quiet
docker compose --env-file /opt/traffoflex/.env.production -f deploy/docker-compose.prod.yml pull
docker compose --env-file /opt/traffoflex/.env.production -f deploy/docker-compose.prod.yml up -d --no-build
docker compose --env-file /opt/traffoflex/.env.production -f deploy/docker-compose.prod.yml ps
```

`traffic-service` returns `/readyz` 503 until at least one valid active
campaign with trafficback has been configured. Configure and verify a campaign
before sending purchased traffic.

### 4. Enable HTTPS routing and renewal

After the application containers are up, enable the three HTTPS vhosts:

```sh
cd /opt/infra
bash /opt/traffoflex/current/render-nginx-prod.sh \
  /opt/traffoflex/current/deploy/infra/nginx/conf.d/traffoflex.conf.example \
  /opt/traffoflex/.env.production > nginx/conf.d/traffoflex.conf
docker compose exec -T nginx nginx -t
docker compose exec -T nginx nginx -s reload
```

The vhosts forward `trk` to traffic-service:8080, `pb` to
postback-service:8081, and `app` `/api/` to api-service:8070 with the remaining
paths served by the static frontend:8080. Docker DNS resolves names on the
shared `proxy` network. Nginx overwrites incoming forwarded IP headers with
its observed peer address. Postback access logs omit the query string because
GET postback URLs can contain secrets.

Check renewal, then install the included host timer. Certbot itself remains a
Docker container:

```sh
cd /opt/infra
docker compose run --rm certbot renew --dry-run
cp /opt/traffoflex/current/deploy/infra/systemd/infra-certbot-renew.* /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now infra-certbot-renew.timer
```

The MongoDB root user is only for administration. On a fresh volume, its init
scripts create indexes and an application user with `readWrite` on `traffoflex`.
ClickHouse starts with an authenticated user and an embedded, persistent
single-node Keeper. Its production image renders the existing SQL to
`ReplicatedMergeTree`/`ReplicatedSummingMergeTree` `_local` tables and
`Distributed` public tables. The initial topology is one shard and one replica.
The single Redpanda broker has topic replication factor 1 and durable fsync;
the six existing topics and event payloads are unchanged.
Its production startup keeps Redpanda host checks enabled. Use a supported
filesystem such as ext4 or XFS for the Docker data directory and resolve any
preflight failures before activation.

For an ordinary restart or status check, run these commands from
`/opt/traffoflex/current`:

```sh
docker compose --env-file /opt/traffoflex/.env.production -f deploy/docker-compose.prod.yml up -d --no-build
docker compose --env-file /opt/traffoflex/.env.production -f deploy/docker-compose.prod.yml ps
docker compose --env-file /opt/traffoflex/.env.production -f deploy/docker-compose.prod.yml logs --tail=100
```

`docker compose down` keeps named volumes; never use `down -v` on production.
The three databases and the traffic cache, Bloom snapshot and click WAL have
persistent volumes. Infra certificates live in `/opt/infra/letsencrypt`.

Keep the host firewall open only for SSH 22 and HTTP/HTTPS 80/443, with other
inbound ports denied. Use SSH keys, `PasswordAuthentication no` and
`PermitRootLogin prohibit-password`; make these host changes manually after
confirming SSH access. Check capacity and container health with `docker stats`,
`df -h`, `free -h`, `iostat`, the Compose `ps` command above and service logs.

## Expansion and data risks

The `traffoflex_cluster` ClickHouse config and the public `Distributed` table
names establish the shard boundary now. For another replica, expand Keeper to
a quorum on separate machines, add a ClickHouse node with a distinct replica
macro and update the cluster topology. A new shard additionally requires a
Kafka consumer allocation, routing and cross-shard query checks. Existing data
does not rebalance automatically. The current application queries use the
public table names, while ingestion and materialized views write local tables;
test the full click/postback/report path when changing the topology.

Do not run this fresh-volume initialization against an existing MongoDB,
ClickHouse or Redpanda volume. MongoDB auth, Redpanda broker IDs and the
ClickHouse table engines differ from development. Take and verify backups,
then plan a controlled migration and backfill if data already exists.
Keeper and all three data services still share one VPS, so this topology has
no host-level high availability.

Before sending purchased traffic, verify admin login and logout through HTTPS,
prove backup and restore, and run the full click/postback smoke and
failure-recovery tests.
The tracker starts independently of MongoDB, ClickHouse and Redpanda. A saved
active routing snapshot permits redirects during their outage; without a usable
snapshot it stays running but reports `/readyz` as unavailable until MongoDB
loads a valid campaign. Allow for the normal refresh interval after recovery.
