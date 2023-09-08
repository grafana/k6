#!/bin/bash

set -eEuo pipefail

eval "$(go env)"

set -x
export OUT_DIR="${1-dist}"
# To override the latest git tag as the version, pass something else as the second arg.
export VERSION=${2:-$(git describe --tags --always --dirty)}

set +x

build() {
    local ALIAS="$1" SUFFIX="${2}"  # Any other arguments are passed to the go build command as env vars
    local NAME="k6-${VERSION}-${ALIAS}"

    local BUILD_ARGS=(-o "${OUT_DIR}/${NAME}/k6${SUFFIX}" -trimpath)

    local PACKAGE_FORMATS
    IFS="," read -ra PACKAGE_FORMATS <<< "${3}"

    local ENV_VARS
    IFS="," read -ra ENV_VARS <<< "${4}"

    echo "- Building platform: ${ALIAS} (" "${ENV_VARS[@]}" "go build" "${BUILD_ARGS[@]}" ")"

    mkdir -p "${OUT_DIR}/${NAME}"

    # Subshell to not mess with the current env vars or CWD
    (
        export "${ENV_VARS[@]}"
        # Build a binary
        go build "${BUILD_ARGS[@]}"

        for format in "${PACKAGE_FORMATS[@]}"; do
            package "$format"
        done
    )
}

package() {
    local FMT="$1"
    echo "- Creating ${NAME}.${FMT} package..."
    case $FMT in
    deb|rpm)
        # nfpm can't substitute env vars in file paths, so we have to cd...
        cd "${OUT_DIR}/${NAME}"
        set -x # Show exactly what command was executed
        nfpm package --config ../../packaging/nfpm.yaml --packager "${FMT}" \
            --target "../k6-${VERSION}-linux-${GOARCH}.${FMT}"
        set +x
        cd -
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

build linux-amd64   ""      tgz,rpm,deb     GOOS=linux,GOARCH=amd64,CGO_ENABLED=0
build linux-arm64   ""      tgz             GOOS=linux,GOARCH=arm64,CGO_ENABLED=0 # TODO: package rpm and dep too
build macos-amd64   ""      zip             GOOS=darwin,GOARCH=amd64
build macos-arm64   ""      zip             GOOS=darwin,GOARCH=arm64
build windows-amd64 .exe    zip             GOOS=windows,GOARCH=amd64
