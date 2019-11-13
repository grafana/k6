#!/usr/bin/env bash

set -eEuo pipefail

eval "$(go env)"

# To override the latest git tag as the version, pass something else as the first arg.
VERSION=${1:-$(git describe --tags --abbrev=0)}

make_archive() {
	local FMT="$1" DIR="$2"

	case $FMT in
	zip)
		zip -rq9 "$DIR.zip" "$DIR"
		;;
	tgz)
		tar -zcf "$DIR.tar.gz" "$DIR"
		;;
	esac
}

build_dist() {
	local ALIAS="$1" GOOS="$2" GOARCH="$3" FMT="${4}" SUFFIX="${5:-}"

	echo "- Building platform: ${ALIAS} (${GOOS} ${GOARCH})"
	local DIR="k6-${VERSION}-${ALIAS}"

	# Clean out any old remnants of failed builds.
	rm -rf "dist/$DIR"
	mkdir -p "dist/$DIR"

	# Build a binary, embed what we can by means of static assets inside it.
	GOARCH="$GOARCH" GOOS="$GOOS" go build -o "dist/$DIR/k6${SUFFIX}"

	# Archive it all, native format depends on the platform. Subshell to not mess with $PWD.
	( cd dist && make_archive "$FMT" "$DIR" )

	# Delete the source files.
	rm -rf "dist/$DIR"
}

checksum() {
	local CHECKSUM_FILE="k6-${VERSION}-checksums.txt"

	if command -v sha256sum > /dev/null; then
		CHECKSUM_CMD="sha256sum"
	elif command -v shasum > /dev/null; then
		CHECKSUM_CMD="shasum -a 256"
	else
		echo "ERROR: unable to find a command to compute sha-256 hash"
		return 1
	fi

	rm -f "dist/$CHECKSUM_FILE"
	( cd dist && for x in *; do "$CHECKSUM_CMD" "$x" >> "$CHECKSUM_FILE"; done )
}

echo "--- Building Release: ${VERSION}"

echo "-> Building platform packages..."
mkdir -p dist

build_dist mac darwin amd64 zip
build_dist win32 windows 386 zip .exe
build_dist win64 windows amd64 zip .exe
build_dist linux32 linux 386 tgz
build_dist linux64 linux amd64 tgz

echo "-> Generating checksum file..."
checksum
