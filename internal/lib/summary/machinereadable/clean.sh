#!/bin/sh -e

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

# Remove the cog-generated files
find "$SCRIPT_DIR" -maxdepth 1 -type f -name '*.go' -exec rm -f {} \;
rm -rf "$SCRIPT_DIR/cog"
rm -rf "$SCRIPT_DIR/docs"

# Remove the schemas repository
rm -rf "$SCRIPT_DIR/k6-summary"
