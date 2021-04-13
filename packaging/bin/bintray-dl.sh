#!/bin/bash
set -eExuo pipefail

PKGDIR="$1"
baseurl='http://dl.bintray.com/loadimpact'

mkdir -p "$PKGDIR" && cd "$_"

for repo in deb rpm msi; do
  files="$(curl -fsSL "${baseurl}/${repo/msi/windows}/" \
    | grep -oP '(?<="nofollow">).*\.'$repo'(?=</a)' \
    | grep -E 'v0\.(2[7-9]|3.)\.')"
  curl -fsSLRZ --remote-name-all $(echo "$files" | sed "s,^,${baseurl}/${repo/msi/windows}/," | paste -sd' ')
done
