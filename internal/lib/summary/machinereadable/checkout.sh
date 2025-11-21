#!/bin/sh -e

sha=e49468be9020f95472c4b7ec51cef179b4fdd1ca # this is just the commit it was last tested with

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

mkdir -p "$SCRIPT_DIR/k6-summary"
cd "$SCRIPT_DIR/k6-summary"

git init
git remote add origin https://github.com/grafana/k6-summary
git fetch origin --depth=1 "${sha}"
git reset --hard "${sha}"

cd -
