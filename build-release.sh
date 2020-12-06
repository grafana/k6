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
    local NAME="k6-${VERSION}-${ALIAS}"

    local BUILD_ENV=("${@:3}")
    local BUILD_ARGS=(-o "${OUT_DIR}/${NAME}/k6${SUFFIX}" -trimpath)

    if [ -n "$VERSION_DETAILS" ]; then
        BUILD_ARGS+=(-ldflags "-X github.com/loadimpact/k6/lib/consts.VersionDetails=$VERSION_DETAILS")
    fi

    echo "- Building platform: ${ALIAS} (" "${BUILD_ENV[@]}" "go build" "${BUILD_ARGS[@]}" ")"

    mkdir -p "${OUT_DIR}/${NAME}"

    # Subshell to not mess with the current env vars or CWD
    (
        export "${BUILD_ENV[@]}"
        # Build a binary
         go build "${BUILD_ARGS[@]}"
    )
}

package() {
    local ALIAS="$1" FMT="$2"
    local NAME="k6-${VERSION}-${ALIAS}"
    echo "- Creating ${NAME}.${FMT} package..."
    case $FMT in
    deb|rpm)
        # The go-bin-* tools expect the binary in /tmp/
        [ ! -r /tmp/k6 ] && cp "${OUT_DIR}/${NAME}/k6" /tmp/k6
        "go-bin-${FMT}" generate --file "packaging/${FMT}.json" -a amd64 \
            --version "${VERSION#v}" -o "${OUT_DIR}/k6-${VERSION}-amd64.${FMT}"
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

CHECKSUM_FILE="k6-${VERSION}-checksums.txt"
checksum() {
    if command -v sha256sum > /dev/null; then
        CHECKSUM_CMD=("sha256sum")
    elif command -v shasum > /dev/null; then
        CHECKSUM_CMD=("shasum" "-a" "256")
    else
        echo "ERROR: unable to find a command to compute sha-256 hash"
        exit 1
    fi

    echo "--- Generating checksum file..."
    rm -f "${OUT_DIR}/$CHECKSUM_FILE"
    (cd "$OUT_DIR" && find . -maxdepth 1 -type f -printf '%P\n' | sort | xargs "${CHECKSUM_CMD[@]}" > "$CHECKSUM_FILE")
}

cleanup() {
    find "$OUT_DIR" -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} \;
    echo "--- Cleaned ${OUT_DIR}"
}
trap cleanup EXIT

echo "--- Building Release: ${VERSION}"

mkdir -p "$OUT_DIR"

build mac     ""   GOOS=darwin  GOARCH=amd64
build win32   .exe GOOS=windows GOARCH=386
build win64   .exe GOOS=windows GOARCH=amd64
build linux32 ""   GOOS=linux   GOARCH=386    CGO_ENABLED=0
build linux64 ""   GOOS=linux   GOARCH=amd64  CGO_ENABLED=0

package linux32 tgz
package linux64 tgz
package linux64 rpm
package linux64 deb
package mac     zip
package win32   zip
package win64   zip

checksum
