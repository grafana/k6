#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="$SCRIPT_DIR/out"

mkdir -p "$OUT_DIR"

k6 run   --js-profiling-enabled   --js-profiling-scope=vu   --js-cpu-profile-output="$OUT_DIR/demo-vu.pprof"   --js-runtime-trace-output="$OUT_DIR/demo-vu.trace"   "$SCRIPT_DIR/script.js"

echo "Wrote:"
echo "  $OUT_DIR/demo-vu.pprof"
echo "  $OUT_DIR/demo-vu.trace"
