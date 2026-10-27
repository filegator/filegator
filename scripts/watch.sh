#!/usr/bin/env bash
# Rebuild and restart the server when sources change.
#
# No external dependencies: it polls file mtimes with find(1) instead of
# requiring air/reflex/entr/fswatch. Run via `make watch`.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

GO="${GO:-go}"

# Same tool-state rules as the Makefile: keep Go's cache and telemetry out of
# the working tree even when HOME points here.
export GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/filegator-gocache}"
export GOMODCACHE="${GOMODCACHE:-${TMPDIR:-/tmp}/filegator-gomodcache}"
export GOPATH="${GOPATH:-${TMPDIR:-/tmp}/filegator-gopath}"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-${TMPDIR:-/tmp}/filegator-xdg-cache}"
export XDG_CONFIG_HOME="${XDG_CONFIG_HOME:-${TMPDIR:-/tmp}/filegator-xdg-config}"
export GOFLAGS="${GOFLAGS:--mod=vendor}"
BINARY="${BINARY:-bin/filegator}"
INTERVAL="${WATCH_INTERVAL:-1}"
ADDR="${ADDR:-127.0.0.1:8080}"
CONFIG="${CONFIG:-private/config.json}"
# Empty means the site root; the server falls back to base_path in the config.
BASE_PATH="${BASE_PATH:-}"

# The frontend is embedded under internal/app/frontend, so any file here needs
# the binary rebuilt and the process restarted.
WATCH=(internal go.mod go.sum)

targets=()
for path in "${WATCH[@]}"; do
  [ -e "$path" ] && targets+=("$path")
done

stamp="$(mktemp)"
child=""

cleanup() {
  rm -f "$stamp"
  if [ -n "$child" ] && kill -0 "$child" 2>/dev/null; then
    kill "$child" 2>/dev/null || true
    wait "$child" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

changed() {
  [ -n "$(find "${targets[@]}" -type f -newer "$stamp" -print -quit 2>/dev/null)" ]
}

build_and_restart() {
  if ! "$GO" build -o "$BINARY" .; then
    echo "[watch] build failed; waiting for changes" >&2
    return 1
  fi
  if [ -n "$child" ] && kill -0 "$child" 2>/dev/null; then
    kill "$child" 2>/dev/null || true
    wait "$child" 2>/dev/null || true
  fi
  echo "[watch] starting $BINARY${BASE_PATH:+ under $BASE_PATH}"
  if [ -n "$BASE_PATH" ]; then
    "./$BINARY" -addr "$ADDR" -config "$CONFIG" -base-path "$BASE_PATH" &
  else
    "./$BINARY" -addr "$ADDR" -config "$CONFIG" &
  fi
  child=$!
}

touch "$stamp"
build_and_restart || true
echo "[watch] watching ${targets[*]} (Ctrl-C to stop)"

while true; do
  sleep "$INTERVAL"
  if changed; then
    touch "$stamp"
    echo "[watch] change detected; rebuilding"
    build_and_restart || true
  fi
done
