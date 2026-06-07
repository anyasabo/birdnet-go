#!/usr/bin/env bash
# Build the fork .deb inside a Linux container and extract it to dist/deb/.
#
# Works on any host with Docker (incl. macOS) -- the binary is CROSS-COMPILED
# inside a native linux/amd64 container, so building an arm64 (Raspberry Pi)
# package needs no qemu emulation and runs at full speed.
#
# Usage:
#   packaging/deb/build-docker.sh            # arm64 (Pi), default
#   ARCH=amd64 packaging/deb/build-docker.sh # x86-64
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

ARCH="${ARCH:-arm64}"
OUTDIR="${OUTDIR:-dist/deb}"

case "$ARCH" in
    arm64|amd64) ;;
    *) echo "unsupported ARCH: $ARCH (expected arm64 or amd64)" >&2; exit 1 ;;
esac

if ! docker buildx version >/dev/null 2>&1; then
    echo "ERROR: docker buildx is required (Docker Desktop ships it)." >&2
    exit 1
fi

mkdir -p "$OUTDIR"

# Compute the version on the HOST: .dockerignore excludes .git, so git tags are
# not visible inside the build container. Pass it (and maintainer) as build-args.
VERSION="${VERSION:-$("$REPO_ROOT/packaging/deb/version.sh")}"
MAINTAINER="${MAINTAINER:-$(git config user.name 2>/dev/null || echo fork) <$(git config user.email 2>/dev/null || echo fork@localhost)>}"

echo ">> Building ${ARCH} .deb ${VERSION} in a linux/amd64 container (cross-compile, no emulation)"

# Pin the builder to linux/amd64 so the Go build runs natively and cross-compiles
# to the target arch, rather than emulating the target under qemu.
docker buildx build \
    --platform linux/amd64 \
    --build-arg ARCH="$ARCH" \
    --build-arg VERSION="$VERSION" \
    --build-arg MAINTAINER="$MAINTAINER" \
    --target export \
    --output "type=local,dest=${OUTDIR}" \
    -f packaging/deb/Dockerfile \
    .

echo ">> Done. Package(s) in ${OUTDIR}/:"
ls -1 "${OUTDIR}"/*.deb 2>/dev/null || { echo "no .deb produced" >&2; exit 1; }
