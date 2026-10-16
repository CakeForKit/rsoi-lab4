#!/usr/bin/env bash

set -euo pipefail

variant=${1:-${VARIANT:-}}
service=${2:-${SERVICE_NAME:-}}
port=${3:-${PORT_NUMBER:-}}

if [[ -z "$variant" || -z "$service" || -z "$port" ]]; then
  echo "Usage: $0 <variant> <service> <port>" >&2
  exit 2
fi

path=$(dirname "$0")
service_stopped=false

cleanup() {
  if [[ "$service_stopped" == true ]]; then
    docker compose start "$service" >/dev/null
  fi
}
trap cleanup EXIT

run_folder() {
  newman run --delay-request=100 --folder="$1" \
    --export-environment "$variant"/postman/environment.json \
    --environment "$variant"/postman/environment.json \
    "$variant"/postman/collection.json
}

step() {
  local number=$1

  if [[ $((number % 2)) -eq 1 ]]; then
    printf "=== Step %d: stop %s ===\n" "$number" "$service"
    docker compose stop "$service"
    service_stopped=true
  else
    printf "=== Step %d: start %s ===\n" "$number" "$service"
    docker compose start "$service"
    "$path"/wait-for.sh -t 120 "http://localhost:$port/manage/health" -- true
    service_stopped=false
  fi

  run_folder "step$number"
}

run_folder success
step 1
step 2
step 3
step 4
