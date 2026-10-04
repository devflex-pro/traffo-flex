#!/usr/bin/env bash
set -Eeuo pipefail

read_env() {
  local key=$1 file=$2
  awk -F= -v key="$key" '$1 == key { sub(/^[^=]*=/, ""); value = $0 } END { print value }' "$file"
}

validate_domain() {
  local name=$1 domain=$2 label
  if (( ${#domain} > 253 )) || [[ ! "$domain" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$ ]]; then
    printf '%s must be a lowercase DNS hostname without scheme, port or path\n' "$name" >&2
    exit 1
  fi
  local -a labels
  IFS=. read -r -a labels <<< "$domain"
  for label in "${labels[@]}"; do
    if (( ${#label} > 63 )); then
      printf '%s contains a DNS label longer than 63 characters\n' "$name" >&2
      exit 1
    fi
  done
}

if [[ "${1:-}" == --validate ]]; then
  [[ $# -eq 4 ]] || { printf 'Usage: render-nginx-prod.sh --validate TRACKER POSTBACK APP\n' >&2; exit 2; }
  tracker_domain=$2
  postback_domain=$3
  app_domain=$4
else
  [[ $# -eq 2 ]] || { printf 'Usage: render-nginx-prod.sh TEMPLATE ENV_FILE\n' >&2; exit 2; }
  template_file=$1
  env_file=$2
  [[ -f "$template_file" && -f "$env_file" ]] || { printf 'Nginx template and production env file are required\n' >&2; exit 1; }
  tracker_domain=$(read_env TRACKER_DOMAIN "$env_file")
  postback_domain=$(read_env POSTBACK_DOMAIN "$env_file")
  app_domain=$(read_env APP_DOMAIN "$env_file")
fi
validate_domain TRACKER_DOMAIN "$tracker_domain"
validate_domain POSTBACK_DOMAIN "$postback_domain"
validate_domain APP_DOMAIN "$app_domain"
if [[ "$tracker_domain" == "$postback_domain" || "$tracker_domain" == "$app_domain" || "$postback_domain" == "$app_domain" ]]; then
  printf 'Tracker, postback and app domains must be distinct\n' >&2
  exit 1
fi

[[ "${1:-}" == --validate ]] && exit 0

template=$(< "$template_file")
[[ "$template" == *'__TRACKER_DOMAIN__'* && "$template" == *'__POSTBACK_DOMAIN__'* && "$template" == *'__APP_DOMAIN__'* ]] || {
  printf 'Nginx template is missing domain placeholders\n' >&2
  exit 1
}
template=${template//__TRACKER_DOMAIN__/$tracker_domain}
template=${template//__POSTBACK_DOMAIN__/$postback_domain}
template=${template//__APP_DOMAIN__/$app_domain}
printf '%s\n' "$template"
