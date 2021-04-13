#!/bin/bash
set -eEuo pipefail

# External dependencies:
# - https://github.com/s3tools/s3cmd
#   s3cmd expects AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY to be set in the
#   environment.
# - generate_index.py
#   For generating the index.html of each directory. It's available in the
#   packaging/bin directory of the k6 repo, and should be in $PATH.

_s3bucket="${S3_BUCKET-dl.k6.io}"
_usage="Usage: $0 <pkgdir> <repodir> [s3bucket=${_s3bucket}]"
PKGDIR="${1?${_usage}}"  # The directory where .msi files are located
REPODIR="${2?${_usage}}" # The package repository working directory
S3PATH="${3-${_s3bucket}}/msi"
# Remove packages older than N number of days (730 is roughly ~2 years).
REMOVE_PKG_DAYS=730

log() {
  echo "$(date -Iseconds) $*"
}

delete_old_pkgs() {
  find "$1" -name '*.msi' -type f -daystart -mtime "+${REMOVE_PKG_DAYS}" -print0 | xargs -r0 rm -v
}

sync_to_s3() {
  log "Syncing to S3 ..."
  s3cmd sync --delete-removed "${REPODIR}/" "s3://${S3PATH}/"

  # Set a short cache expiration for index files and the latest MSI package.
  s3cmd modify --recursive --exclude='*' \
    --include='index.html' --include='k6-latest-amd64.msi' \
    --add-header='Cache-Control: max-age=60,must-revalidate' "s3://${S3PATH}/"
}

mkdir -p "$REPODIR"

# Download existing packages
# For MSI packages this is only done to be able to generate the index.html correctly.
# Should we fake it and create empty files that have the same timestamp and size as the original ones?
s3cmd sync --exclude='*' --include='*.msi' "s3://${S3PATH}/" "$REPODIR/"

# Copy the new packages in
find "$PKGDIR" -name "*.msi" -type f -print0 | xargs -r0 cp -avt "$REPODIR"

delete_old_pkgs "$REPODIR"

# Update the latest package. This could be done with S3 redirects, but
# CloudFront caches redirects aggressively and I wasn't able to invalidate it.
latest="$(find "$REPODIR" -name '*.msi' -printf '%P\n' | sort | tail -1)"
cp -p "${REPODIR}/${latest}" "${REPODIR}/k6-latest-amd64.msi"

log "Generating index.html ..."
(cd "$REPODIR" && generate_index.py -r)

sync_to_s3
