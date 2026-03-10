#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="$SCRIPT_DIR/out"
MODE="${1:-parallel}"

mkdir -p "$OUT_DIR"

if [[ "$MODE" == "single" ]]; then
  k6 run     --js-profiling-enabled     --js-profiling-scope=vu     --js-cpu-profile-output="$OUT_DIR/demo-browser-single.pprof"     --js-runtime-trace-output="$OUT_DIR/demo-browser-single.trace"     "$SCRIPT_DIR/single.js"

  echo "Wrote:"
  echo "  $OUT_DIR/demo-browser-single.pprof"
  echo "  $OUT_DIR/demo-browser-single.trace"
  exit 0
fi

k6 run   --js-profiling-enabled   --js-profiling-scope=vu   --js-cpu-profile-output="$OUT_DIR/demo-browser-isolation.pprof"   --js-runtime-trace-output="$OUT_DIR/demo-browser-isolation.trace"   "$SCRIPT_DIR/parallel.js"

echo "Wrote:"
echo "  $OUT_DIR/demo-browser-isolation.pprof"
echo "  $OUT_DIR/demo-browser-isolation.trace"
