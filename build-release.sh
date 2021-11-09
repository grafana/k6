#!/bin/bash

set -eEuo pipefail

eval "$(go env)"

OUT_DIR="${1-dist}"
# To override the latest git tag as the version, pass something else as the second arg.
VERSION=${2:-$(git describe --tags --always --dirty)}

# To overwrite the version details, pass something as the third arg. Empty string disables it.
VERSION_DETAILS=${3-"$(date -u +"%FT%T%z")/$(git describe --always --long --dirty)"}

build() {
    local ALIAS="$1" SUFFIX="${2}"  # Any other arguments are passed to the go build command as env vars
    local NAME="xk6-browser-${VERSION}-${ALIAS}"

    local BUILD_ENV=("${@:3}")
    local BUILD_ARGS=(--output "${OUT_DIR}/${NAME}/xk6-browser${SUFFIX}")

    #if [ -n "$VERSION_DETAILS" ]; then
    #    BUILD_ARGS+=(-ldflags "-X go.k6.io/k6/lib/consts.VersionDetails=$VERSION_DETAILS")
    #fi

    echo "- Building platform: ${ALIAS} (" "${BUILD_ENV[@]}" "go build" "${BUILD_ARGS[@]}" ")"

    mkdir -p "${OUT_DIR}/${NAME}"

    # Subshell to not mess with the current env vars or CWD
    (
        export "${BUILD_ENV[@]}"
        # Build a binary
        xk6 build --with github.com/grafana/xk6-browser=. "${BUILD_ARGS[@]}"
    )
}

package() {
    local ALIAS="$1" FMT="$2"
    local NAME="xk6-browser-${VERSION}-${ALIAS}"
    echo "- Creating ${NAME}.${FMT} package..."
    case $FMT in
    deb|rpm)
        # The go-bin-* tools expect the binary in /tmp/
        [ ! -r /tmp/xk6-browser ] && cp "${OUT_DIR}/${NAME}/xk6-browser" /tmp/xk6-browser
        "go-bin-${FMT}" generate --file "packaging/${FMT}.json" -a amd64 \
            --version "${VERSION#v}" -o "${OUT_DIR}/${NAME}.${FMT}"
        ;;
    tgz)
        tar -C "${OUT_DIR}" -zcf "${OUT_DIR}/${NAME}.tar.gz" "$NAME"
        ;;
    zip)
        (cd "${OUT_DIR}" && zip -rq9 - "$NAME") > "${OUT_DIR}/${NAME}.zip"
        ;;
    *)
        echo "Unknown format: $FMT"
        return 1
        ;;
    esac
}

cleanup() {
    find "$OUT_DIR" -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} \;
    echo "--- Cleaned ${OUT_DIR}"
}
trap cleanup EXIT

echo "--- Building Release: ${VERSION}"

mkdir -p "$OUT_DIR"

build macos-amd64   ""   GOOS=darwin  GOARCH=amd64
build macos-arm64   ""   GOOS=darwin  GOARCH=arm64
build windows-amd64 .exe GOOS=windows GOARCH=amd64
build linux-amd64   ""   GOOS=linux   GOARCH=amd64  CGO_ENABLED=0
build linux-arm64   ""   GOOS=linux   GOARCH=arm64  CGO_ENABLED=0

package macos-amd64   zip
package macos-arm64   zip
package windows-amd64 zip
package linux-amd64   tgz
package linux-arm64   tgz
package linux-amd64   rpm
package linux-amd64   deb
