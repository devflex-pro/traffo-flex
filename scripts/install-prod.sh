#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'EOF'
Usage: install-prod.sh --version latest|vX.Y.Z

Install a published TraffoFlex release on a fresh Debian 13 host with Docker
Engine and the Docker Compose plugin already installed. Run as root from a
terminal. Existing credentials, certificates, volumes and shared infra files
are preserved. Re-running the same version is supported; upgrades need a
separate, reviewed operation.
EOF
}

die() {
  printf 'Install failed: %s\n' "$*" >&2
  exit 1
}

info() {
  printf '\n==> %s\n' "$*"
}

ask() {
  local label=$1 value
  printf '%s: ' "$label" >/dev/tty
  IFS= read -r value </dev/tty || die 'Cannot read from the terminal'
  printf '%s' "$value"
}

ask_secret() {
  local label=$1 value='' char tty_settings
  tty_settings=$(stty -g </dev/tty) || die 'Cannot inspect terminal settings'
  stty -echo </dev/tty || die 'Cannot hide terminal input'
  trap 'stty "$tty_settings" </dev/tty' EXIT
  printf '%s: ' "$label" >/dev/tty
  while true; do
    IFS= read -r -n 1 char </dev/tty || die 'Cannot read from the terminal'
    if [[ -z "$char" ]]; then
      break
    fi
    if [[ "$char" == $'\177' || "$char" == $'\b' ]]; then
      if [[ -n "$value" ]]; then
        value=${value%?}
        printf '\b \b' >/dev/tty
      fi
      continue
    fi
    value+=$char
    printf '*' >/dev/tty
  done
  stty "$tty_settings" </dev/tty
  trap - EXIT
  printf '\n' >/dev/tty
  printf '%s' "$value"
}

read_env() {
  local key=$1 file=$2
  awk -F= -v key="$key" '$1 == key { sub(/^[^=]*=/, ""); value = $0 } END { print value }' "$file"
}

random_hex() {
  od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
}

require_safe_value() {
  local label=$1 value=$2
  [[ "$value" =~ ^[A-Za-z0-9._@%+-]+$ ]] || die "$label must contain only URL-safe characters"
}

release=''
case "${1:-}" in
  --help|-h)
    usage
    exit 0
    ;;
  --version)
    [[ $# -eq 2 ]] || die 'Pass exactly --version latest or --version vX.Y.Z'
    release=$2
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
if [[ "$release" != latest && ! "$release" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9]+)*$ ]]; then
  die 'Release must be latest or a vX.Y.Z tag'
fi
[[ $EUID -eq 0 ]] || die 'Run as root, for example with sudo bash'
[[ -r /dev/tty && -w /dev/tty ]] || die 'An interactive terminal is required'

for tool in docker curl awk od tr cmp mktemp systemctl findmnt stty; do
  command -v "$tool" >/dev/null 2>&1 || die "Missing host command: $tool"
done
[[ -f /etc/os-release ]] || die 'Cannot identify the host OS'
# shellcheck disable=SC1091
source /etc/os-release
[[ "${ID:-}" == debian && "${VERSION_ID:-}" == 13 ]] || die 'This installer targets Debian 13'
[[ "$(uname -m)" == x86_64 ]] || die 'This installer targets x86_64'
docker compose version >/dev/null 2>&1 || die 'Docker Compose plugin is required'
docker info >/dev/null 2>&1 || die 'Docker Engine is not running or is unavailable'
docker_root=$(docker info --format '{{.DockerRootDir}}')
docker_fs=$(findmnt -no FSTYPE --target "$docker_root")
[[ "$docker_fs" == ext4 || "$docker_fs" == xfs ]] || die "Docker data is on $docker_fs; use ext4 or XFS for Redpanda"

base=/opt/traffoflex
infra=/opt/infra
env_file="$base/.env.production"
current="$base/current"
proxy_subnet=172.30.240.0/24
nginx_ip=172.30.240.10
anonymous_docker_config=$(mktemp -d)
stage=''
bundle_container=''

cleanup() {
  if [[ -n "$bundle_container" ]]; then
    docker rm "$bundle_container" >/dev/null 2>&1 || true
  fi
  if [[ -n "$stage" && -d "$stage" ]]; then
    rm -rf -- "$stage"
  fi
  rmdir -- "$anonymous_docker_config" 2>/dev/null || true
}
trap cleanup EXIT

if [[ "$release" == latest ]]; then
  info 'Resolving the latest published release'
  latest_image=ghcr.io/devflex-pro/traffo-flex-deploy-bundle:latest
  docker --config "$anonymous_docker_config" pull "$latest_image" || \
    die 'The latest deployment bundle is not publicly available'
  release=$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.version"}}' "$latest_image")
  [[ "$release" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9]+)*$ ]] || \
    die 'The latest deployment bundle has no valid release version label'
  info "Selected $release"
fi
release_dir="$base/releases/$release"
bundle_image="ghcr.io/devflex-pro/traffo-flex-deploy-bundle:$release"

if [[ -e "$current" && ! -L "$current" ]]; then
  die "$current already exists and is not an installer-managed symlink"
fi
if [[ -L "$current" ]]; then
  installed=$(basename -- "$(readlink -- "$current")")
  [[ "$installed" == "$release" ]] || die "Installed release is $installed; this installer does not upgrade it automatically"
fi
if [[ -e "$base/deploy" && ! -L "$current" ]]; then
  die "$base contains a manual installation; move it before using this installer"
fi
if [[ ! -L "$current" ]]; then
  for volume in traffoflex_mongo_data traffoflex_clickhouse_data traffoflex_redpanda_data traffoflex_traffic_bloom_data traffoflex_traffic_cache_data traffoflex_traffic_click_wal_data; do
    if docker volume inspect "$volume" >/dev/null 2>&1; then
      die "Existing production volume $volume needs a reviewed migration before installation"
    fi
  done
fi
if [[ -L "$release_dir" || ( -d "$release_dir" && ! -f "$release_dir/README.md" ) ]]; then
  die "$release_dir is not a complete installer-managed release directory"
fi

info "Checking public GHCR images for $release"
for service in api-service traffic-service postback-service admin-frontend clickhouse deploy-bundle; do
  image="ghcr.io/devflex-pro/traffo-flex-$service:$release"
  docker --config "$anonymous_docker_config" manifest inspect "$image" >/dev/null || \
    die "Cannot pull $image anonymously; publish the GHCR package or check the release tag"
done

mkdir -p "$base/releases" "$infra" /opt/backups/traffoflex
chmod 700 "$base"

if [[ ! -f "$release_dir/README.md" ]]; then
  info "Downloading deployment bundle $release"
  docker --config "$anonymous_docker_config" pull "$bundle_image"
  stage=$(mktemp -d "$base/releases/.install.$release.XXXXXX")
  bundle_container=$(docker create "$bundle_image")
  docker cp "$bundle_container:/bundle/." "$stage/"
  docker rm "$bundle_container" >/dev/null
  bundle_container=''
  mv -- "$stage" "$release_dir"
  stage=''
fi
for file in README.md CHANGELOG.md install-prod.sh render-nginx-prod.sh .env.production.example deploy/docker-compose.prod.yml deploy/infra/compose.yml deploy/infra/.env.example deploy/infra/nginx/conf.d/traffoflex.conf.example deploy/mongo/init/001_indexes.js deploy/mongo/prod-init/002_app_user.js; do
  [[ -f "$release_dir/$file" ]] || die "Deployment bundle is missing $file"
done
nginx_template="$release_dir/deploy/infra/nginx/conf.d/traffoflex.conf.example"
nginx_renderer="$release_dir/render-nginx-prod.sh"
if [[ ! -L "$current" ]]; then
  ln -s "releases/$release" "$current"
fi

if [[ ! -f "$env_file" ]]; then
  info 'Configuring the first installation'
  tracker_domain=$(ask 'Tracker domain (for example trk.example.com)')
  postback_domain=$(ask 'Postback domain (for example pb.example.com)')
  app_domain=$(ask 'Admin domain (for example app.example.com)')
  bash "$nginx_renderer" --validate "$tracker_domain" "$postback_domain" "$app_domain" || die 'Invalid production domains'
  admin_email=$(ask 'Admin email (receives login codes)')
  sender_email=$(ask 'Sender email on a Resend-verified domain (for example auth@example.com)')
  resend_key=$(ask_secret 'Resend API key')
  require_safe_value 'Admin email' "$admin_email"
  require_safe_value 'Sender email' "$sender_email"
  require_safe_value 'Resend API key' "$resend_key"
  [[ "$admin_email" == *@* && "$sender_email" == *@* ]] || die 'Both email values must be email addresses'
  root_password=$(random_hex)
  app_password=$(random_hex)
  clickhouse_password=$(random_hex)
  jwt_secret=$(random_hex)
  original_umask=$(umask)
  umask 077
  cat > "$env_file" <<EOF
IMAGE_TAG=$release
TRACKER_DOMAIN=$tracker_domain
POSTBACK_DOMAIN=$postback_domain
APP_DOMAIN=$app_domain
MONGO_ROOT_USER=traffoflex_root
MONGO_ROOT_PASSWORD=$root_password
MONGO_APP_USER=traffoflex_app
MONGO_APP_PASSWORD=$app_password
CLICKHOUSE_USER=traffoflex
CLICKHOUSE_PASSWORD=$clickhouse_password
AUTH_ADMIN_EMAIL=$admin_email
AUTH_JWT_SECRET=$jwt_secret
AUTH_EMAIL_FROM=$sender_email
AUTH_RESEND_API_KEY=$resend_key
NGINX_PROXY_CIDR=$nginx_ip/32
EOF
  umask "$original_umask"
  unset root_password app_password clickhouse_password jwt_secret resend_key
else
  [[ "$(read_env IMAGE_TAG "$env_file")" == "$release" ]] || die 'Existing .env.production uses a different IMAGE_TAG'
  [[ "$(read_env NGINX_PROXY_CIDR "$env_file")" == "$nginx_ip/32" ]] || die 'Existing trusted proxy CIDR differs from the installer default'
  chmod 600 "$env_file"
fi
tracker_domain=$(read_env TRACKER_DOMAIN "$env_file")
postback_domain=$(read_env POSTBACK_DOMAIN "$env_file")
app_domain=$(read_env APP_DOMAIN "$env_file")
bash "$nginx_renderer" --validate "$tracker_domain" "$postback_domain" "$app_domain" || die 'Invalid production domains'
unset TRACKER_DOMAIN POSTBACK_DOMAIN APP_DOMAIN

if docker network inspect proxy >/dev/null 2>&1; then
  existing_subnet=$(docker network inspect -f '{{(index .IPAM.Config 0).Subnet}}' proxy)
  [[ "$existing_subnet" == "$proxy_subnet" ]] || die "Existing proxy subnet is $existing_subnet; expected $proxy_subnet"
else
  info 'Creating the shared proxy network'
  docker network create --driver bridge --subnet "$proxy_subnet" proxy
fi

if [[ -e "$infra/compose.yml" ]]; then
  for file in compose.yml nginx/nginx.conf nginx/conf.d/00-default.conf; do
    cmp -s "$infra/$file" "$release_dir/deploy/infra/$file" || die "Shared infra file $infra/$file differs; review it manually"
  done
else
  info 'Installing shared Nginx and Certbot configuration'
  cp -a "$release_dir/deploy/infra/." "$infra/"
fi
if [[ ! -f "$infra/.env" ]]; then
  cp "$infra/.env.example" "$infra/.env"
  chmod 600 "$infra/.env"
fi
[[ "$(read_env NGINX_PROXY_IP "$infra/.env")" == "$nginx_ip" ]] || die 'Existing infra NGINX_PROXY_IP differs from the trusted proxy address'
mkdir -p "$infra/letsencrypt" "$infra/certbot-www"
mkdir -p "$infra/certbot-www/.well-known/acme-challenge"
chmod 755 "$infra/certbot-www" "$infra/certbot-www/.well-known" "$infra/certbot-www/.well-known/acme-challenge"
nginx_config="$infra/nginx/conf.d/traffoflex.conf"
if [[ -e "$nginx_config" ]]; then
  cmp -s "$nginx_config" <(bash "$nginx_renderer" "$nginx_template" "$env_file") || \
    die "$nginx_config differs from the configured domains; review the certificate and domain change manually"
elif [[ -e "$infra/letsencrypt/live/traffoflex/fullchain.pem" ]]; then
  die 'An existing TraffoFlex certificate has no Nginx config; review its domains manually before continuing'
fi

info 'Starting shared Nginx and obtaining TLS certificates'
docker compose --env-file "$infra/.env" -f "$infra/compose.yml" config --quiet
docker compose --env-file "$infra/.env" -f "$infra/compose.yml" up -d nginx
if [[ ! -f "$infra/letsencrypt/live/traffoflex/fullchain.pem" || ! -f "$infra/letsencrypt/live/traffoflex/privkey.pem" ]]; then
  cert_email=$(read_env AUTH_ADMIN_EMAIL "$env_file")
  docker compose --env-file "$infra/.env" -f "$infra/compose.yml" run --rm certbot certonly \
    --non-interactive --webroot -w /var/www/certbot \
    --cert-name traffoflex --email "$cert_email" --agree-tos --no-eff-email \
    -d "$tracker_domain" -d "$postback_domain" -d "$app_domain" </dev/null
fi
if [[ ! -e "$nginx_config" ]]; then
  bash "$nginx_renderer" "$nginx_template" "$env_file" > "$nginx_config"
fi

info 'Starting TraffoFlex'
docker compose --env-file "$env_file" -f "$release_dir/deploy/docker-compose.prod.yml" config --quiet
docker compose --env-file "$env_file" -f "$release_dir/deploy/docker-compose.prod.yml" pull
docker compose --env-file "$env_file" -f "$release_dir/deploy/docker-compose.prod.yml" up -d --no-build

docker compose --env-file "$infra/.env" -f "$infra/compose.yml" exec -T nginx nginx -t </dev/null
docker compose --env-file "$infra/.env" -f "$infra/compose.yml" exec -T nginx nginx -s reload </dev/null

admin_ready=false
for ((attempt = 0; attempt < 12; attempt++)); do
  if curl -fsS --noproxy '*' --resolve "$app_domain:443:127.0.0.1" "https://$app_domain/" >/dev/null 2>&1; then
    admin_ready=true
    break
  fi
  sleep 5
done
[[ "$admin_ready" == true ]] || die 'Admin HTTPS route did not become ready; inspect the Nginx and application logs'

install -m 0644 "$release_dir/deploy/infra/systemd/infra-certbot-renew.service" /etc/systemd/system/
install -m 0644 "$release_dir/deploy/infra/systemd/infra-certbot-renew.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now infra-certbot-renew.timer

info 'Installation completed'
docker compose --env-file "$env_file" -f "$release_dir/deploy/docker-compose.prod.yml" ps
printf '\nAdmin: https://%s\nTracker: https://%s\nPostbacks: https://%s\n' "$app_domain" "$tracker_domain" "$postback_domain"
printf 'Configure an active campaign with trafficback before sending paid traffic.\n'
printf 'Production activation still requires verified backups and full smoke/failure tests.\n'
printf 'Follow %s/README.md for validation and operations.\n' "$release_dir"
