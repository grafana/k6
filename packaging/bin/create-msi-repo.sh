#!/bin/bash
set -eEuo pipefail

# External dependencies:
# - https://github.com/s3tools/s3cmd
#   s3cmd expects AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY to be set in the
#   environment.
# - generate_index.py
#   For generating the index.html of each directory. It's available in the
#   packaging/bin directory of the k6 repo, and should be in $PATH.

_s3bucket="${S3_BUCKET-dl-k6-io}"
_usage="Usage: $0 <pkgdir> <repodir> [s3bucket=${_s3bucket}]"
PKGDIR="${1?${_usage}}"  # The directory where .msi files are located
REPODIR="${2?${_usage}}" # The package repository working directory
S3PATH="${3-${_s3bucket}}/msi"
# Number of packages to keep, older packages will be deleted.
RETAIN_PKG_COUNT=25

log() {
    echo "$(date -Iseconds) $*"
}

delete_old_pkgs() {
  find "$1" -name '*.msi' -type f | sort -r | tail -n "+$((RETAIN_PKG_COUNT+1))" | xargs -r rm -v
}

sync_to_s3() {
  log "Syncing to S3 ..."
  s3cmd sync --delete-removed "${REPODIR}/" "s3://${S3PATH}/"

  # Disable cache for index files.
  s3cmd modify --recursive --exclude='*' --include='index.html' \
    --add-header='Cache-Control:no-cache, max-age=0' "s3://${S3PATH}/"
}

mkdir -p "$REPODIR"

# Download existing packages
# For MSI packages this is only done to be able to generate the index.html correctly.
# Should we fake it and create empty files that have the same timestamp and size as the original ones?
s3cmd sync --exclude='*' --include='*.msi' "s3://${S3PATH}/" "$REPODIR/"

# Copy the new packages in
find "$PKGDIR" -name "*.msi" -type f -print0 | xargs -r0 cp -t "$REPODIR"

# TODO: Handle k6-latest-amd64.msi

delete_old_pkgs "$REPODIR"

log "Generating index.html ..."
(cd "$REPODIR" && generate_index.py -r)

sync_to_s3
