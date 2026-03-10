#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="$SCRIPT_DIR/out"

mkdir -p "$OUT_DIR"
"$SCRIPT_DIR/../generate_data.sh" json "$SCRIPT_DIR/data.json" 6

k6 run \
  --js-profiling-enabled \
  --js-profiling-scope=combined \
  --js-cpu-profile-output="$OUT_DIR/demo-json-shared.pprof" \
  --js-runtime-trace-output="$OUT_DIR/demo-json-shared.trace" \
  "$SCRIPT_DIR/script.js"
