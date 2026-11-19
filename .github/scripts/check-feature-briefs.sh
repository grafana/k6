#!/usr/bin/env bash
# check-feature-briefs.sh — validate feature briefs in docs/features/
#
# Usage:
#   ./check-feature-briefs.sh               # check all briefs (local default)
#   ./check-feature-briefs.sh path/to/a.md  # check specific file(s) (used by CI)
#
# Rules enforced:
#   1. Total prose ≤ 1,200 words   (hard limit, error)
#      Total prose ≤   800 words   (soft target, warning)
#   2. "## Problem" section ≤ 3 sentences  (heuristic: counts . ! ? terminators)
#   3. "## Cost of inaction" section must be present and non-empty
#
# Prose is defined as all text after stripping:
#   - Markdown heading lines  (lines starting with #)
#   - HTML comments           (<!-- ... -->, including multi-line)
#
# README.md and 000-TEMPLATE.md are always excluded.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BRIEFS_DIR="$REPO_ROOT/docs/features"

SKIP_PATTERN="README\.md$|000-TEMPLATE\.md$"

# ──────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────

# strip_comments FILE — print file contents with HTML comments removed
strip_comments() {
  awk '
    {
      line = $0
      while (1) {
        if (in_comment) {
          idx = index(line, "-->")
          if (idx == 0) { line = ""; break }
          line = substr(line, idx + 3)
          in_comment = 0
        } else {
          idx = index(line, "<!--")
          if (idx == 0) break
          before = substr(line, 1, idx - 1)
          rest   = substr(line, idx + 4)
          end_idx = index(rest, "-->")
          if (end_idx > 0) {
            line = before substr(rest, end_idx + 3)
          } else {
            printf "%s", before
            in_comment = 1
            line = ""
            break
          }
        }
      }
      print line
    }
  ' in_comment=0 "$1"
}

# prose_text FILE — file with comments and heading lines stripped
prose_text() {
  strip_comments "$1" | grep -v '^[[:space:]]*#'
}

# section_text FILE HEADING — text of the section starting at HEADING, up to next ## heading
section_text() {
  local file="$1"
  local heading="$2"
  strip_comments "$file" | awk -v heading="$heading" '
    BEGIN { found=0 }
    /^##[[:space:]]/ {
      if (found) exit
      if (tolower($0) ~ tolower(heading)) { found=1; next }
    }
    found { print }
  '
}

# ──────────────────────────────────────────────
# Build the file list
# ──────────────────────────────────────────────

if [[ $# -gt 0 ]]; then
  files=("$@")
else
  # No args: scan all .md files in docs/features/
  mapfile -t files < <(find "$BRIEFS_DIR" -maxdepth 1 -name '*.md' | sort)
fi

# Filter out excluded files
filtered=()
for f in "${files[@]}"; do
  if echo "$f" | grep -qE "$SKIP_PATTERN"; then
    continue
  fi
  if [[ ! -f "$f" ]]; then
    echo "::warning::File not found, skipping: $f"
    continue
  fi
  filtered+=("$f")
done

if [[ ${#filtered[@]} -eq 0 ]]; then
  echo "No feature briefs to check."
  exit 0
fi

# ──────────────────────────────────────────────
# Validate each brief
# ──────────────────────────────────────────────

failed=0

for f in "${filtered[@]}"; do
  echo "Checking: $f"

  # 1. Word count -----------------------------------------------------------
  word_count=$(prose_text "$f" | wc -w | tr -d '[:space:]')

  if [[ "$word_count" -gt 1200 ]]; then
    echo "::error file=$f::Word count is $word_count (limit: 1200). Trim the prose."
    failed=1
  elif [[ "$word_count" -gt 800 ]]; then
    echo "::warning file=$f::Word count is $word_count (soft target: ≤800). Consider trimming."
  fi

  # 2. Problem sentence count -----------------------------------------------
  # Heuristic: count sentence-terminating punctuation (. ! ?) that is followed
  # by whitespace or end of string. HTML comments already stripped by section_text.
  problem_text=$(section_text "$f" "## Problem")
  sentence_count=$(echo "$problem_text" | grep -oE '[.!?]([[:space:]]|$)' | wc -l | tr -d '[:space:]')

  if [[ "$sentence_count" -gt 3 ]]; then
    echo "::error file=$f::Problem statement has ~$sentence_count sentences (limit: 3)."
    failed=1
  fi

  # 3. Cost of inaction — present and non-empty -----------------------------
  coi_text=$(section_text "$f" "## Cost of inaction" | tr -d '[:space:]')

  if [[ -z "$coi_text" ]]; then
    echo "::error file=$f::\"Cost of inaction\" section is missing or empty. It is required."
    failed=1
  fi

done

if [[ "$failed" -ne 0 ]]; then
  exit 1
fi

echo "All briefs OK."
