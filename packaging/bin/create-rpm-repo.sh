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

_s3bucket="${S3_BUCKET-dl-k6-io}"
_usage="Usage: $0 <pkgdir> <repodir> [s3bucket=${_s3bucket}]"
PKGDIR="${1?${_usage}}"  # The directory where .rpm files are located
REPODIR="${2?${_usage}}" # The package repository working directory
S3PATH="${3-${_s3bucket}}/rpm"

architectures="x86_64"

# TODO: Remove old package versions?
# Something like: https://github.com/kopia/kopia/blob/master/tools/apt-publish.sh#L23-L25

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
done
