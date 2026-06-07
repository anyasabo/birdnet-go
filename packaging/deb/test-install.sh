#!/usr/bin/env bash
# Smoke-test the built .deb by actually installing it in an arm64 Debian
# container (under qemu emulation) and exercising the install.
#
# Unlike the build, this MUST emulate the target arch: we run the arm64
# maintainer scripts and exec the arm64 binary to prove the vendored shared
# libraries resolve and the CGO links work. It's a short run, so emulation is fine.
#
# What it verifies:
#   - apt resolves the declared Depends (ffmpeg, sox, libsox-fmt-mp3, libasound2)
#   - postinst runs: creates the birdnet-go user, runs ldconfig
#   - the vendored libs register with the dynamic linker
#   - the arm64 binary actually execs (i.e. its .so deps load) via `--help`
#
# Note: systemd isn't PID 1 in the container, so the postinst systemctl steps
# are skipped by its `[ -d /run/systemd/system ]` guard (expected, not a failure).
#
# Usage:
#   packaging/deb/test-install.sh                 # arm64, newest deb in dist/deb
#   ARCH=amd64 packaging/deb/test-install.sh
#   DEB=path/to.deb packaging/deb/test-install.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

ARCH="${ARCH:-arm64}"
PLATFORM="linux/${ARCH}"
IMAGE="${IMAGE:-debian:trixie-slim}" # matches the runtime base in the upstream Dockerfile

# Pick the .deb: explicit $DEB, else newest matching the arch (portable: no
# GNU-only find -printf, since this runs on the macOS host).
if [ -z "${DEB:-}" ]; then
    for f in dist/deb/*_"${ARCH}".deb; do
        [ -e "$f" ] || continue
        if [ -z "${DEB:-}" ] || [ "$f" -nt "$DEB" ]; then
            DEB="$f"
        fi
    done
fi
if [ -z "${DEB:-}" ] || [ ! -f "$DEB" ]; then
    echo "ERROR: no .deb found (looked for dist/deb/*_${ARCH}.deb). Build one first:" >&2
    echo "       ARCH=${ARCH} packaging/deb/build-docker.sh" >&2
    exit 1
fi
DEB_DIR="$(cd "$(dirname "$DEB")" && pwd)"
DEB_FILE="$(basename "$DEB")"

echo ">> Testing install of ${DEB_FILE} on ${PLATFORM} (${IMAGE})"

docker run --rm --platform "$PLATFORM" \
    -v "${DEB_DIR}:/pkg:ro" \
    -e "DEB_FILE=${DEB_FILE}" \
    "$IMAGE" bash -euo pipefail -c '
        echo "== container arch: $(uname -m) =="

        export DEBIAN_FRONTEND=noninteractive
        apt-get update -q

        echo "== apt install (resolves declared Depends) =="
        apt-get install -y -q "/pkg/${DEB_FILE}"

        echo "== installed file list =="
        dpkg -L birdnet-go | grep -E "bin/birdnet-go|/usr/lib/birdnet-go|config.yaml|\.service|ld.so.conf.d" || true

        echo "== service user created by postinst? =="
        getent passwd birdnet-go && echo "user OK" || { echo "FAIL: birdnet-go user missing"; exit 1; }

        echo "== vendored libs registered with linker? =="
        ldconfig -p | grep -E "tensorflowlite_c|onnxruntime" || { echo "FAIL: libs not in linker cache"; exit 1; }

        echo "== binary execs (its .so deps load)? =="
        if birdnet-go --help >/dev/null 2>&1; then
            echo "binary OK"
        else
            echo "FAIL: birdnet-go --help did not run cleanly:" >&2
            birdnet-go --help || true
            ldd "$(command -v birdnet-go)" || true
            exit 1
        fi

        echo "== ALL CHECKS PASSED =="
    '
