#!/bin/bash
set -eEuo pipefail

# External dependencies:
# - https://github.com/rpm-software-management/createrepo
# - https://github.com/s3tools/s3cmd
#   s3cmd expects AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY to be set in the
#   environment.
# - https://gnupg.org/
#   For signing the script expects the private signing key to already be
#   imported and the `rpm` command configured for signing, e.g. ~/.rpmmacros
#   should exist.
# - generate_index.py
#   For generating the index.html of each directory. It's available in the
#   packaging/bin directory of the k6 repo, and should be in $PATH.

_s3bucket="${S3_BUCKET-dl.k6.io}"
_usage="Usage: $0 <pkgdir> <repodir> [s3bucket=${_s3bucket}]"
PKGDIR="${1?${_usage}}"  # The directory where .rpm files are located
REPODIR="${2?${_usage}}" # The package repository working directory
S3PATH="${3-${_s3bucket}}/rpm"
# Number of packages to keep for each architecture, older packages will be deleted.
RETAIN_PKG_COUNT=25

log() {
    echo "$(date -Iseconds) $*"
}

delete_old_pkgs() {
  find "$1" -name '*.rpm' -type f | sort -r | tail -n "+$((RETAIN_PKG_COUNT+1))" | xargs -r rm -v
}

sync_to_s3() {
  log "Syncing to S3 ..."
  s3cmd sync --delete-removed "${REPODIR}/" "s3://${S3PATH}/"

  # Disable cache for repo metadata and index files, so that new releases will be
  # available immediately.
  s3cmd --recursive modify --add-header="Cache-Control:no-cache, max-age=0" \
    "s3://${S3PATH}/x86_64/repodata"
  s3cmd modify --recursive --exclude='*' --include='index.html' \
    --add-header='Cache-Control:no-cache, max-age=0' "s3://${S3PATH}/"
}

architectures="x86_64"

pushd . > /dev/null
mkdir -p "$REPODIR" && cd "$_"

for arch in $architectures; do
  mkdir -p "$arch" && cd "$_"

  # Download existing packages
  s3cmd sync --exclude='*' --include='*.rpm' "s3://${S3PATH}/${arch}/" ./

  # Copy the new packages in and generate signatures
  # FIXME: The architecture naming used by yum docs and in public RPM repos is
  # "x86_64", whereas our packages are named with "amd64". So we do a replacement
  # here, but we should probably consider naming them with "x86_64" instead.
  find "$PKGDIR" -name "*${arch/x86_64/amd64}*.rpm" -type f -print0 | while read -r -d $'\0' f; do
    cp -av "$f" "$PWD/"
    rpm --addsign "${f##*/}"
  done
  createrepo .
  cd -

  delete_old_pkgs "$arch"
done

log "Generating index.html ..."
generate_index.py -r

popd > /dev/null
sync_to_s3
