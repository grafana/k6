#!/bin/bash

set -e

# To override the latest git tag as the version, pass something else as the first arg.
VERSION=${1:-$(git describe --tags --abbrev=0)}

# Fail early if external dependencies aren't installed.
node --version > /dev/null || (echo "ERROR: node is not installed, bailing out."; exit 1)
rice --help > /dev/null || (echo "ERROR: rice is not installed, run: go get github.com/GeertJohan/go.rice"; exit 1)

make_archive() {
	FMT=$1
	DIR=$2

	case $FMT in
	zip)
		zip -rq9 $DIR.zip $DIR
		;;
	tgz)
		tar -zcf $DIR.tar.gz $DIR
		;;
	esac
}

build_dist() {
	ALIAS=$1
	GOOS=$2
	GOARCH=$3
	FMT=$4
	SUFFIX=$5

	echo "- Building platform: ${ALIAS} (${GOOS} ${GOARCH})"
	DIR=k6-${VERSION}-${ALIAS}
	BIN=k6${SUFFIX}

	# Clean out any old remnants of failed builds.
	rm -rf dist/$DIR
	mkdir -p dist/$DIR

	# Build a binary, embed what we can by means of static assets inside it.
	GOARCH=$GOARCH GOOS=$GOOS go build -i -o dist/$DIR/$BIN
	rice append --exec=dist/$DIR/$BIN -i ./api -i ./js -i ./js/compiler
	mkdir -p dist/$DIR/js && cp -R js/node_modules dist/$DIR/js

	# Archive it all, native format depends on the platform. Subshell to not mess with $PWD.
	(
		cd dist
		make_archive $FMT $DIR
		rm -rf $DIR
	)
}

echo "--- Building Release: ${VERSION}"

echo "-> Building web assets..."
make web

echo "-> Building platform packages..."
mkdir -p dist

build_dist mac darwin amd64 zip
build_dist win32 windows 386 zip .exe
build_dist win64 windows amd64 zip .exe
build_dist linux32 linux 386 tgz
build_dist linux64 linux amd64 tgz
