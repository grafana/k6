#!/bin/bash
set -eEuo pipefail

# External dependencies:
# - https://salsa.debian.org/apt-team/apt (apt-ftparchive, packaged in apt-utils)
# - https://github.com/s3tools/s3cmd
#   s3cmd expects AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY to be set in the
#   environment.
# - https://gnupg.org/
#   For signing the script expects the private signing key to already be
#   imported and PGPKEYID and PGP_SIGN_KEY_PASSPHRASE to be set in the
#   environment.
# - generate_index.py
#   For generating the index.html of each directory. It's available in the
#   packaging/bin directory of the k6 repo, and should be in $PATH.

_s3bucket="${S3_BUCKET-dl.k6.io}"
_usage="Usage: $0 <pkgdir> <repodir> [s3bucket=${_s3bucket}]"
PKGDIR="${1?${_usage}}"  # The directory where .deb files are located
REPODIR="${2?${_usage}}" # The package repository working directory
S3PATH="${3-${_s3bucket}}/deb"
# Number of packages to keep for each architecture, older packages will be deleted.
RETAIN_PKG_COUNT=25

log() {
    echo "$(date -Iseconds) $*"
}

delete_old_pkgs() {
  find "$1" -name '*.deb' -type f | sort -r | tail -n "+$((RETAIN_PKG_COUNT+1))" | xargs -r rm -v
  # Remove any dangling .asc files
  find "$1" -name '*.asc' -type f -print0 | while read -r -d $'\0' f; do
    if ! [ -r "${f%.*}" ]; then
      rm -v "$f"
    fi
  done
}

sync_to_s3() {
  log "Syncing to S3 ..."
  s3cmd sync --delete-removed "${REPODIR}/" "s3://${S3PATH}/"

  # Disable cache for repo metadata and index files, so that new releases will be
  # available immediately.
  s3cmd modify --add-header="Cache-Control:no-cache, max-age=0" \
    "s3://${S3PATH}/dists/stable/"{Release,Release.gpg,InRelease}
  s3cmd modify --add-header="Cache-Control:no-cache, max-age=0" \
    "s3://${S3PATH}/dists/stable/main/binary-amd64"/Packages{,.gz,.bz2}
  s3cmd modify --recursive --exclude='*' --include='index.html' \
    --add-header='Cache-Control:no-cache, max-age=0' "s3://${S3PATH}/"
}

# We don't publish i386 packages, but the repo structure is needed for
# compatibility on some systems. See https://unix.stackexchange.com/a/272916 .
architectures="amd64 i386"

pushd . > /dev/null
mkdir -p "$REPODIR" && cd "$_"

for arch in $architectures; do
  bindir="dists/stable/main/binary-$arch"
  mkdir -p "$bindir"
  # Download existing files
  s3cmd sync --exclude='*' --include='*.deb' --include='*.asc' \
    "s3://${S3PATH}/${bindir}/" "$bindir/"

  # Copy the new packages in
  find "$PKGDIR" -name "*$arch*.deb" -type f -print0 | xargs -r0 cp -t "$bindir"
  # Generate signatures for files that don't have it
  # TODO: Switch to debsign instead? This is currently done as Bintray did it,
  # but the signature is not validated by apt/dpkg.
  # https://blog.packagecloud.io/eng/2014/10/28/howto-gpg-sign-verify-deb-packages-apt-repositories/
  find "$bindir" -type f -name '*.deb' -print0 | while read -r -d $'\0' f; do
    if ! [ -r "${f}.asc" ]; then
      gpg2 --default-key="$PGPKEYID" --passphrase="$PGP_SIGN_KEY_PASSPHRASE" \
        --pinentry-mode=loopback --yes --detach-sign --armor -o "${f}.asc" "$f"
    fi
  done
  apt-ftparchive packages "$bindir" | tee "$bindir/Packages"
  gzip -fk "$bindir/Packages"
  bzip2 -fk "$bindir/Packages"

  delete_old_pkgs "$bindir"
done

log "Creating release file..."
apt-ftparchive release \
  -o APT::FTPArchive::Release::Origin="k6" \
  -o APT::FTPArchive::Release::Label="k6" \
  -o APT::FTPArchive::Release::Suite="stable" \
  -o APT::FTPArchive::Release::Codename="stable" \
  -o APT::FTPArchive::Release::Architectures="$architectures" \
  -o APT::FTPArchive::Release::Components="main" \
  -o APT::FTPArchive::Release::Date="$(date -Ru)" \
  "dists/stable" > "dists/stable/Release"

# Sign release file
gpg2 --default-key="$PGPKEYID" --passphrase="$PGP_SIGN_KEY_PASSPHRASE" \
  --pinentry-mode=loopback --yes --detach-sign --armor \
  -o "dists/stable/Release.gpg" "dists/stable/Release"
gpg2 --default-key="$PGPKEYID" --passphrase="$PGP_SIGN_KEY_PASSPHRASE" \
  --pinentry-mode=loopback --yes --clear-sign \
  -o "dists/stable/InRelease" "dists/stable/Release"

log "Generating index.html ..."
generate_index.py -r

popd > /dev/null
sync_to_s3
