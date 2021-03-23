#!/bin/bash
set -eEuo pipefail

log() {
    echo "$(date -Iseconds) $*"
}

signkeypath="$PWD/sign-key.gpg"
s3bucket="${S3_BUCKET-dl-k6-io}"

if ! [ -r "$signkeypath" ]; then
  log "ERROR: Signing key not found at '$signkeypath'"
  exit 1
fi

gpg2 --import --batch --passphrase="$PGP_SIGN_KEY_PASSPHRASE" "$signkeypath"
export PGPKEYID="$(gpg2 --list-secret-keys --with-colons | grep '^sec' | cut -d: -f5)"
# TODO: Publish the pub key to S3

# Setup RPM signing
cat > "$HOME/.rpmmacros" <<EOF
%_gpgbin        $(which gpg2)
%_gpg_path      $HOME/.gnupg
%_gpg_name      k6
%_gpg_pass      -
%__gpg_sign_cmd   %{__gpg} gpg2 --default-key="$PGPKEYID" --no-verbose --no-armor --pinentry-mode=loopback --yes --passphrase="$PGP_SIGN_KEY_PASSPHRASE" --no-secmem-warning --detach-sign -o %{__signature_filename} %{__plaintext_filename}
EOF

for repo in deb rpm msi; do
  log "Creating ${repo} repository ..."
  "create-${repo}-repo.sh" "$PWD/dist" "$PWD/Packages/${repo}"
done

log "Generating index.html ..."
(cd Packages && generate_index.py -r)

log "Syncing to S3 ..."
s3cmd sync ./Packages/ "s3://${s3bucket}/"

# Disable cache for repo metadata, so that new releases will be available
# immediately.
# TODO: Maybe do this inside each script?
# TODO: How to handle k6-latest-amd64.msi? Could it be an S3 redirect that is never cached?
s3cmd modify --add-header="Cache-Control:no-cache, max-age=0" \
  "s3://${s3bucket}/deb/dists/stable/"{Release,Release.gpg,InRelease}
s3cmd modify --add-header="Cache-Control:no-cache, max-age=0" \
  "s3://${s3bucket}/deb/dists/stable/main/binary-amd64"/Packages{,.gz,.bz2}
s3cmd --recursive modify --add-header="Cache-Control:no-cache, max-age=0" \
  "s3://${s3bucket}/rpm/x86_64/repodata"
s3cmd modify --recursive --exclude='*' --include='index.html' \
  --add-header='Cache-Control:no-cache, max-age=0' "s3://${s3bucket}/"

exec "$@"
