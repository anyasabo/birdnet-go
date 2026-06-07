#!/usr/bin/env bash
# Print the fork .deb version string:
#   <upstream-v-tag>+fork.<utc-date>.g<short-sha>   e.g. 0.6.4+fork.20260606.gb194ecc5
#
# Derived from the latest upstream v* tag (the floating nightly-* tag is ignored)
# so every resync/rebuild yields a strictly higher, apt-upgradeable version.
#
# Shared by build.sh (native/CI) and build-docker.sh. The Docker build derives
# the version on the HOST and passes it in, because .dockerignore excludes .git
# so git is unavailable inside the build container.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

UPSTREAM_VER="$(git describe --tags --match 'v*' --abbrev=0 2>/dev/null || echo 0.0.0)"
UPSTREAM_VER="${UPSTREAM_VER#v}"
BUILD_DATE="$(date -u +%Y%m%d)"
SHORT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"

echo "${UPSTREAM_VER}+fork.${BUILD_DATE}.g${SHORT_SHA}"
