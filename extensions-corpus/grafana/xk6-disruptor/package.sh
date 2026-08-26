#!/bin/bash
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

BUILD="build"
CONFIG="${SCRIPT_DIR}/packaging/nfpm.yaml"
DIST="dist"
NAME="xk6-disruptor"
ARCH=$(go env GOARCH)
OS=$(go env GOOS)

function usage() {
cat << EOF

creates a package for a target architecture and operating system

$0 [OPTIONS] COMMAND

Commands:
  build Builds the binary for the given architecture and operating system
  pack  Packages the binary for the given architecture and operating system in the given format.
  all   Builds and packs all supported combinations of architecture, operating system and package format

options:
  -a, --arch: target architecture (valid option amd64, arm64. Defaults to GOARCH)
  -b, --build: directory for building binaries (defaults to 'build. Created if it does not exist)
  -c, --config: nfpm config file (defaults to packaging/nfpm.yaml) 
  -d, --dist: directory to place packages (defaults to 'dist'. Created if it does not exist)
  -n, --name: package base name. Defaults to 'xk6-disruptor'
  -o, --os: target operating systems (valid options linux, darwing, windows. Defaults to GOOS)
  -p, --pkg: package format (valid options: deb, rpm, tgz)
  -r, --release: release version used for packages (can be different from version for development releases)
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
    -c|--config)
      CONFIG="$2"
      shift
      ;;
    -d|--dist)
      DIST="$2"
      shift
      ;;
    -o|--os)
      OS="$2"
      if [[ ! $OS =~ linux|darwin|windows ]]; then
        error "supported operating systems are 'linux', 'darwin' and 'windows'"
      fi
      shift
      ;;
    -p|--pkg)
      PKG="$2"
      if [[ ! $PKG =~ deb|rpm|tgz|zip ]]; then
        error "supported package formats are  'deb', 'rpm', 'tgz' and 'zip'"
      fi
      shift
      ;;
    -r|--release)
      RELEASE="$2"
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

if [[ ! -e $DIST ]]; then
   mkdir -p $DIST
fi

## make all paths absolute
BUILD=$(realpath $BUILD)
DIST=$(realpath $DIST)
CONFIG=$(realpath $CONFIG)


if [[ -z $OS ]]; then
  error "target operating system is required"
fi

if [[ -z $ARCH ]]; then
  error "target architecture is required"
fi

if [[ -z $PKG ]]; then
  error "package format is required"
fi

if [[ -z $RELEASE ]]; then
  error "release is required"
fi

if [[ -z $BINARY ]]; then
  BINARY=$NAME-$OS-$ARCH
fi

case $PKG in
  deb|rpm)
    if [[ ! $OS == "linux" ]]; then
      error "unsoported operating system for package format $PKG: $OS"
    fi

    if [[ ! $ARCH == "amd64" ]]; then
      error "unsoported architecture for package format $PKG: $ARCH"
    fi

    (
      # nfpm does not support variable substitution in paths so we must run in the build directory
      pushd $BUILD

      export PKG_VERSION=$RELEASE
      TARGET="$DIST/$NAME-$RELEASE-$OS-$ARCH.$PKG"
      nfpm package --config $CONFIG --packager $PKG --target $TARGET

      popd
    )
    ;;
  tgz)
    tar -zcf "$DIST/$NAME-${RELEASE}-${OS}-${ARCH}.tar.gz" -C $BUILD $BINARY
    ;;
  zip)
    zip -rq9j "$DIST/$NAME-${RELEASE}-${OS}-${ARCH}.zip" "$BUILD/$BINARY"
  ;;
  *)
    error "invalid package format only 'deb', 'rpm' and 'tgz' are accepted"
esac
