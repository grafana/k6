#!/bin/bash

set -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

BUILD="build"
NAME="xk6-disruptor"
ARCH=$(go env GOARCH)
OS=$(go env GOOS)

function usage() {
cat << EOF

builds a binary for a target architecture and operating system from a given module version.

$0 [OPTIONS] 

options:
  -a, --arch: target architecture (valid option amd64, arm64. Defaults to GOARCH)
  -b, --build: directory for building binaries (defaults to 'build. Created if it does not exist)
  -k, --k6-version: version of k6 to use
  -n, --name: package base name. Defaults to 'xk6-disruptor'
  -o, --os: target operating systems (valid options linux, darwing, windows. Defaults to GOOS)
  -r, --replace: module that replaces xk6-distruptor module
  -v, --version: xk6-disruptor version in semver format
  -y, --binary: name of the binary (default is name-os-arch)

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
    -a|--arch)
      ARCH="$2"
      if [[ ! $ARCH =~ amd64|arm64 ]]; then
        error "supported architectures are 'amd64' and 'arm64'"
      fi
      shift 
      ;;
    -b|--build)
      BUILD="$2"
      shift
      ;;
    -k|--k6-version)
      K6_VERSION="$2"
      shift
      ;;
    -o|--os)
      OS="$2"
      if [[ ! $OS =~ linux|darwin|windows ]]; then
        error "supported operating systems are 'linux', 'darwin' and 'windows'"
      fi
      shift
      ;;
    -r|--replace)
      REPLACE_MOD="$2"
      shift
      ;;
    -v|--version)
      VERSION="$2"
      shift
      ;;
    -y|--binary)
      BINARY="$2"
      shift
      ;;
    *)
      error "Unknown option $1"
      ;;
  esac
  shift
done

if [[ ! -e $BUILD ]]; then
   mkdir -p $BUILD
fi

## make all paths absolute
BUILD=$(realpath $BUILD)

if [[ -z $OS ]]; then
  error "target operating system is required"
fi

if [[ -z $ARCH ]]; then
  error "target architecture is required"
fi

if [[ -n $REPLACE_MOD && -z $VERSION ]]; then
  error "replace module must be versioned. Version option missing"
fi

if [[ -z $BINARY ]]; then
  BINARY="$NAME-$OS-$ARCH"
fi

# set disruptor version to use for build
MOD=$(go list -m)
REPLACE="."
if [[ -n $VERSION ]]; then
  REPLACE_MOD=${REPLACE_MOD:-$MOD}
  REPLACE=${REPLACE_MOD}@${VERSION}
fi

#start sub shell to create its own environment
(
  if [[ $OS == "linux" ]]; then # disable cross-compiling for linux
    export CGO_ENABLED=0
  fi

  export GOARCH=$ARCH
  export GOOS=$OS
  xk6 build $K6_VERSION --with $MOD=${REPLACE} --output $BUILD/$BINARY
)

