#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <csv|json> <output_file> [target_mb]" >&2
  exit 2
fi

KIND="$1"
OUT_FILE="$2"
TARGET_MB="${3:-6}"
TARGET_BYTES=$((TARGET_MB * 1024 * 1024))

mkdir -p "$(dirname "$OUT_FILE")"

payload() {
  printf 'segment-%0180d' 0
}

write_csv() {
  : > "$OUT_FILE"
  echo 'id,name,email,payload,group' > "$OUT_FILE"
  local i=0
  local size=0
  local p
  p="$(payload)"
  while true; do
    i=$((i + 1))
    printf '%d,user%d,user%d@example.com,%s-%d,g%d\n' "$i" "$i" "$i" "$p" "$((i % 97))" "$((i % 10))" >> "$OUT_FILE"
    size=$(stat -c%s "$OUT_FILE")
    if [[ "$size" -ge "$TARGET_BYTES" ]]; then
      break
    fi
  done
}

write_json() {
  : > "$OUT_FILE"
  echo -n '[' > "$OUT_FILE"
  local i=0
  local first=1
  local size=0
  local p
  p="$(payload)"
  while true; do
    i=$((i + 1))
    if [[ "$first" -eq 0 ]]; then
      echo -n ',' >> "$OUT_FILE"
    fi
    first=0
    printf '{"id":%d,"name":"user%d","email":"user%d@example.com","group":%d,"payload":"%s-%d"}' "$i" "$i" "$i" "$((i % 10))" "$p" "$((i % 101))" >> "$OUT_FILE"
    size=$(stat -c%s "$OUT_FILE")
    if [[ "$size" -ge "$TARGET_BYTES" ]]; then
      break
    fi
  done
  echo -n ']' >> "$OUT_FILE"
}

case "$KIND" in
  csv)
    write_csv
    ;;
  json)
    write_json
    ;;
  *)
    echo "Unsupported kind: $KIND (expected csv or json)" >&2
    exit 2
    ;;
esac

echo "Generated $KIND data: $OUT_FILE ($(stat -c%s "$OUT_FILE") bytes)"
