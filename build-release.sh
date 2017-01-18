#!/bin/bash

set -e

# To override the latest git tag as the version, pass something else as the first arg.
VERSION=${1:-$(git describe --tags --abbrev=0)}

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
	DIR=dist/k6-${VERSION}-${ALIAS}
	rm -rf $DIR
	mkdir -p $DIR

	GOARCH=$GOARCH GOOS=$GOOS go build -i -o $DIR/k6${SUFFIX}
	mkdir -p $DIR/web && cp -R web/dist $DIR/web
	mkdir -p $DIR/js && cp -R js/lib js/node_modules $DIR/js

	make_archive $FMT $DIR
	rm -rf $DIR
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
