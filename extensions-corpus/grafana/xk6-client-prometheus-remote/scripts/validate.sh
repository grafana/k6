#!/usr/bin/env bash
#
# End-to-end validation of the xk6-client-prometheus-remote extension against a
# real Prometheus instance running in Docker.
#
# Steps:
#   1. Build a k6 binary with the extension (via xk6).
#   2. Start Prometheus with the remote write receiver enabled.
#   3. Run scripts/validate.js to remote-write a uniquely named metric.
#   4. Query Prometheus to confirm the metric landed.
#
# Useful after a Prometheus dependency bump to catch protocol/wire regressions.
#
# Usage: scripts/validate.sh
#
# Env overrides:
#   PROM_IMAGE   Prometheus image           (default: prom/prometheus:latest)
#   PROM_PORT    Host port for Prometheus   (default: 9090)
#   K6_BIN       Existing k6 binary to use  (default: build one with xk6)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

PROM_IMAGE="${PROM_IMAGE:-prom/prometheus:latest}"
PROM_PORT="${PROM_PORT:-9090}"
CONTAINER="xk6-prw-validate"
METRIC="xk6_validate_metric"
K6_BIN="${K6_BIN:-}"

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
err() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; }

cleanup() {
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
    if [ -n "${BUILT_K6:-}" ]; then rm -f "$BUILT_K6"; fi
}
trap cleanup EXIT

# 1. Build k6 with the extension (unless a binary was supplied).
if [ -z "$K6_BIN" ]; then
    log "Building k6 with the extension via xk6"
    xk6 build --with "github.com/grafana/xk6-client-prometheus-remote=$REPO_ROOT" >/dev/null
    K6_BIN="$REPO_ROOT/k6"
    BUILT_K6="$K6_BIN"
fi
[ -x "$K6_BIN" ] || { err "k6 binary not found/executable: $K6_BIN"; exit 1; }

# 2. Start Prometheus with the remote write receiver enabled.
log "Starting Prometheus ($PROM_IMAGE) on port $PROM_PORT"
docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$CONTAINER" -p "$PROM_PORT:9090" "$PROM_IMAGE" \
    --config.file=/etc/prometheus/prometheus.yml \
    --web.enable-remote-write-receiver >/dev/null

log "Waiting for Prometheus to become ready"
for _ in $(seq 1 30); do
    if curl -sf "http://localhost:$PROM_PORT/-/ready" >/dev/null 2>&1; then break; fi
    sleep 1
done
curl -sf "http://localhost:$PROM_PORT/-/ready" >/dev/null || { err "Prometheus never became ready"; exit 1; }

# 3. Remote-write a metric.
log "Running k6 remote write script"
REMOTE_WRITE_URL="http://localhost:$PROM_PORT/api/v1/write" \
    "$K6_BIN" run "$REPO_ROOT/scripts/validate.js"

# 4. Confirm the metric is queryable.
log "Querying Prometheus for '$METRIC'"
sleep 2
RESULT="$(curl -sf "http://localhost:$PROM_PORT/api/v1/query?query=$METRIC")"
COUNT="$(printf '%s' "$RESULT" | grep -o '"__name__"' | wc -l | tr -d ' ')"

if [ "$COUNT" -gt 0 ]; then
    log "SUCCESS: found $COUNT series for '$METRIC'"
    printf '%s\n' "$RESULT"
else
    err "metric '$METRIC' not found in Prometheus"
    printf '%s\n' "$RESULT" >&2
    exit 1
fi
