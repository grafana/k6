#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="$SCRIPT_DIR/out"

mkdir -p "$OUT_DIR"

k6 run   --address localhost:6565   --js-profiling-enabled   --js-profiling-scope=combined   "$SCRIPT_DIR/script.js"   >"$OUT_DIR/k6.log" 2>&1 &

K6_PID=$!
cleanup() {
  kill "$K6_PID" 2>/dev/null || true
}
trap cleanup EXIT

sleep 2

curl -fsS http://localhost:6565/v1/js-observability/profiling >"$OUT_DIR/profiling-before.json"
curl -fsS -X POST http://localhost:6565/v1/js-observability/profiling/enable >"$OUT_DIR/profiling-enable.json"
curl -fsS http://localhost:6565/v1/js-observability >"$OUT_DIR/js-observability.json"
curl -fsS "http://localhost:6565/debug/pprof/js-cpu?scope=combined" --output "$OUT_DIR/demo-rest-combined.pprof"
curl -fsS -X POST http://localhost:6565/v1/js-observability/profiling/disable >"$OUT_DIR/profiling-disable.json"

wait "$K6_PID" || true
trap - EXIT

echo "Wrote:"
echo "  $OUT_DIR/profiling-before.json"
echo "  $OUT_DIR/profiling-enable.json"
echo "  $OUT_DIR/js-observability.json"
echo "  $OUT_DIR/demo-rest-combined.pprof"
echo "  $OUT_DIR/profiling-disable.json"
echo "  $OUT_DIR/k6.log"
