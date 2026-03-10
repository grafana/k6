#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="$SCRIPT_DIR/out"

mkdir -p "$OUT_DIR"

k6 run   --address localhost:6565   --js-profiling-enabled   --js-profiling-scope=init   --js-cpu-profile-output="$OUT_DIR/demo-init.pprof"   --js-runtime-trace-output="$OUT_DIR/demo-init.trace"   --js-first-runner-mem-max-bytes=500mb   --js-first-runner-mem-step-percent=5   "$SCRIPT_DIR/script.js"

echo "Wrote:"
echo "  $OUT_DIR/demo-init.pprof"
echo "  $OUT_DIR/demo-init.trace"
