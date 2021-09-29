#!/bin/bash
set -eEuo pipefail

# Generate repositories for k6 packages and sync them from/to AWS S3.
# These scripts use both s3cmd and aws-cli, since the latter doesn't
# preserve timestamps and doesn't compute/check hashes, relying instead
# on timestamps or file size to determine whether a sync should be done.
# aws-cli is still used for invalidating the CloudFront cache, which
# doesn't work with s3cmd...
# See:
# - https://github.com/aws/aws-cli/issues/3069#issuecomment-818732309
# - https://github.com/s3tools/s3cmd/issues/536
# - https://github.com/s3tools/s3cmd/issues/790

log() {
  echo "$(date -Iseconds) $*"
}

signkeypath="$PWD/sign-key.gpg"
s3bucket="${S3_BUCKET-dl.k6.io}"
pkgdir="$PWD/Packages"

if ! [ -r "$signkeypath" ]; then
  log "ERROR: Signing key not found at '$signkeypath'"
  exit 1
fi

gpg2 --import --batch --passphrase="$PGP_SIGN_KEY_PASSPHRASE" "$signkeypath"
export PGPKEYID="$(gpg2 --list-secret-keys --with-colons | grep '^sec' | cut -d: -f5)"
# Export and sync the GPG pub key if it doesn't exist in S3 already.
mkdir -p "$pkgdir"
s3cmd get "s3://${s3bucket}/key.gpg" "${pkgdir}/key.gpg" || {
  gpg2 --export --armor --output "${pkgdir}/key.gpg" "$PGPKEYID"
  s3cmd put "${pkgdir}/key.gpg" "s3://${s3bucket}/key.gpg"
}

for repo in deb rpm msi; do
  log "Creating ${repo} repository ..."
  "create-${repo}-repo.sh" "$PWD/dist" "${pkgdir}/${repo}"
done

# Generate and sync the main index.html
(cd "$pkgdir" && generate_index.py)
s3cmd put --add-header='Cache-Control: max-age=60,must-revalidate' \
  "${pkgdir}/index.html" "s3://${s3bucket}/index.html"

# Invalidate CloudFront cache for index files, repo metadata and the latest MSI package.
IFS=' ' read -ra indexes <<< \
  "$(find "${pkgdir}" -name 'index.html' -type f | sed "s:^${pkgdir}::" | sort | paste -sd' ')"
aws cloudfront create-invalidation --distribution-id "$AWS_CF_DISTRIBUTION" \
  --paths "${indexes[@]}" "/msi/k6-latest-amd64.msi" \
  "/deb/dists/stable/"{Release,Release.gpg,InRelease} \
  "/deb/dists/stable/main/binary-amd64"/Packages{,.gz,.bz2} \
  "/rpm/x86_64/repodata/*"

exec "$@"
