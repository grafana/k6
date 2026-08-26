#!/bin/bash

set -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
BUILD=$SCRIPT_DIR/build.sh
PACKAGE=$SCRIPT_DIR/package.sh

function usage() {
cat << EOF

builds binary and creates installation packages for all supported platforms

$0 [OPTIONS]

options:
  -r, --release: release version used for packages (can be different from version for development releases)
  -v, --version: xk6-disruptor version in semver format

EOF
}

# Prints an error message the usage help and exits with error
function error () {
   echo $1
   usage
   exit 1
}

while [[ $# -gt 0 ]]; do
  case $1 in
    -r|--release)
      RELEASE="$2"
      shift
      ;;
    -v|--version)
      VERSION="-v $2"
      shift
      ;;
    *)
      error "Unknown option $1"
      ;;
  esac
  shift
done

if [[ -z $RELEASE ]]; then
  error "release is required"
fi

$BUILD -o linux -a amd64 $VERSION
$BUILD -o linux -a arm64 $VERSION
$BUILD -o darwin -a amd64 $VERSION
$BUILD -o darwin -a arm64 $VERSION
$BUILD -o windows -a amd64 $VERSION -y xk6-disruptor-windows-amd64.exe
$PACKAGE -o linux -a amd64 -p deb -r $RELEASE
$PACKAGE -o linux -a amd64 -p rpm -r $RELEASE
$PACKAGE -o linux -a amd64 -p tgz -r $RELEASE
$PACKAGE -o linux -a arm64 -p tgz -r $RELEASE
$PACKAGE -o darwin -a amd64 -p tgz -r $RELEASE
$PACKAGE -o darwin -a arm64 -p tgz -r $RELEASE
$PACKAGE -o windows -a amd64 -p zip -r $RELEASE -y xk6-disruptor-windows-amd64.exe
