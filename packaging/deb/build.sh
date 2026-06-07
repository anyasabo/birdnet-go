#!/usr/bin/env bash
# Build a fork-maintained native .deb for BirdNET-Go.
#
# This script is deliberately self-contained and only *calls* upstream's build
# targets -- it never edits Taskfile.yml or the release workflow. That keeps the
# fork resync (git rebase upstream/main) conflict-free: everything here lives in
# net-new files under packaging/deb/.
#
# Usage:
#   packaging/deb/build.sh                # arm64 (Raspberry Pi 64-bit), default
#   ARCH=amd64 packaging/deb/build.sh     # x86-64
#
# Env overrides:
#   ARCH        deb arch: arm64 | amd64        (default arm64)
#   MAINTAINER  Debian maintainer string       (default from git config)
#   OUTDIR      where the .deb is written       (default dist/deb)
#
# Output: $OUTDIR/birdnet-go_<version>_<arch>.deb
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

ARCH="${ARCH:-arm64}"
OUTDIR="${OUTDIR:-dist/deb}"
STAGING="$REPO_ROOT/packaging/deb/staging"

# Map the deb arch to the Go arch / upstream Taskfile target and the directory
# where its CGO toolchain drops the shared libraries.
case "$ARCH" in
    arm64)
        TASK_TARGET="linux_arm64"
        # Cross-compiler lib dir on the CI host (see release-build.yml). When
        # building natively on a Pi the libs land in the system lib dir instead;
        # both are in LIB_SEARCH below so either works.
        LIB_SEARCH="/usr/aarch64-linux-gnu/lib /usr/lib /usr/local/lib"
        ONNX_ARCH="aarch64" # microsoft/onnxruntime release naming
        ;;
    amd64)
        TASK_TARGET="linux_amd64"
        LIB_SEARCH="/usr/lib /usr/local/lib"
        ONNX_ARCH="x64"
        ;;
    *)
        echo "unsupported ARCH: $ARCH (expected arm64 or amd64)" >&2
        exit 1
        ;;
esac

MAINTAINER="${MAINTAINER:-$(git config user.name 2>/dev/null || echo fork) <$(git config user.email 2>/dev/null || echo fork@localhost)>}"

# --- Version: upstream semver tag + fork metadata ---------------------------
# Derived by version.sh (shared with build-docker.sh). May be overridden via the
# VERSION env var -- the Docker build does this because .git is unavailable
# inside the build container, so the version is computed on the host and passed in.
VERSION="${VERSION:-$("$REPO_ROOT/packaging/deb/version.sh")}"

echo ">> Building birdnet-go ${VERSION} for ${ARCH} (target: ${TASK_TARGET})"

# --- 1. Build the binary via upstream's own target -------------------------
# Cross-compiles the embedded binary (frontend + models baked in) and downloads
# the matching TFLite library. BUILD_VERSION is baked into the binary so it
# reports the fork version at runtime / in support dumps.
export BUILD_VERSION="$VERSION"
task "$TASK_TARGET"

# --- 2. Stage artifacts ----------------------------------------------------
rm -rf "$STAGING"
mkdir -p "$STAGING"

cp bin/birdnet-go "$STAGING/birdnet-go"

# Ship upstream's current default config verbatim, so the packaged config.yaml
# always tracks upstream without a separately maintained copy.
cp internal/conf/config.yaml "$STAGING/config.yaml"

find_lib() {
    lib_name="$1"
    for dir in $LIB_SEARCH; do
        # Prefer an exact match, fall back to a versioned soname.
        if [ -f "$dir/$lib_name" ]; then
            echo "$dir/$lib_name"
            return 0
        fi
        for match in "$dir/$lib_name".*; do
            if [ -f "$match" ]; then
                echo "$match"
                return 0
            fi
        done
    done
    echo "ERROR: could not find $lib_name in: $LIB_SEARCH" >&2
    echo "       (did 'task $TASK_TARGET' run? check release-build.yml for current lib paths)" >&2
    return 1
}

# TFLite is downloaded by the task above into the target-arch cross lib dir.
cp "$(find_lib libtensorflowlite_c.so)" "$STAGING/libtensorflowlite_c.so"

# ONNX Runtime is NOT fetched by the build target, and `task download-onnxruntime`
# keys off the build-host arch (wrong for a cross build). So fetch the correct
# TARGET-arch library directly from upstream's pinned release. Version is read
# from the Taskfile so it tracks upstream without a hardcoded copy here.
ONNX_VER="$(sed -nE "s/^[[:space:]]*ONNXRUNTIME_VERSION:[[:space:]]*'?([0-9.]+)'?.*/\1/p" Taskfile.yml | head -n1)"
if [ -z "$ONNX_VER" ]; then
    echo "ERROR: could not read ONNXRUNTIME_VERSION from Taskfile.yml" >&2
    exit 1
fi
echo ">> Fetching ONNX Runtime ${ONNX_VER} (${ONNX_ARCH})"
ONNX_TMP="$(mktemp -d)"
trap 'rm -rf "$ONNX_TMP"' EXIT
curl -fsSL "https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VER}/onnxruntime-linux-${ONNX_ARCH}-${ONNX_VER}.tgz" \
    -o "$ONNX_TMP/onnx.tgz"
tar -xzf "$ONNX_TMP/onnx.tgz" -C "$ONNX_TMP" --strip-components=1
# Ship the concrete soname target, normalized to the bare name the binary loads.
ONNX_SO="$(find "$ONNX_TMP/lib" -name 'libonnxruntime.so*' | sort | head -n1)"
if [ -z "$ONNX_SO" ]; then
    echo "ERROR: libonnxruntime.so not found in downloaded archive" >&2
    exit 1
fi
cp "$ONNX_SO" "$STAGING/libonnxruntime.so"

# --- 3. Package ------------------------------------------------------------
if ! command -v nfpm >/dev/null 2>&1; then
    echo ">> nfpm not found; installing via 'go install'..."
    go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
    GOPATH_BIN="$(go env GOPATH)/bin"
    export PATH="$PATH:$GOPATH_BIN"
fi

mkdir -p "$OUTDIR"
export ARCH VERSION MAINTAINER
nfpm pkg \
    --config packaging/deb/nfpm.yaml \
    --packager deb \
    --target "$OUTDIR/birdnet-go_${VERSION}_${ARCH}.deb"

echo ">> Built $OUTDIR/birdnet-go_${VERSION}_${ARCH}.deb"
