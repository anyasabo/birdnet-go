# Local/CI builder for the fork-maintained .deb.
#
# Runs entirely on the BUILD platform (linux/amd64 under Docker Desktop on
# macOS) and CROSS-COMPILES to the target arch via gcc-aarch64-linux-gnu -- no
# qemu emulation, so an arm64 (Raspberry Pi) package builds at native speed.
#
# Usage is wrapped by packaging/deb/build-docker.sh, which extracts the .deb to
# dist/deb/ via buildx --output. The mirrors the buildenv in the upstream
# Dockerfile but stays a separate, net-new file so fork resyncs never conflict.
#
#   ARCH        target deb arch: arm64 (default) | amd64
#   VERSION     fork deb version (computed on the host; .git is dockerignored)
#   MAINTAINER  Debian maintainer string
ARG ARCH=arm64

FROM --platform=$BUILDPLATFORM golang:1.26-trixie AS builder
ARG ARCH
ARG VERSION
ARG MAINTAINER

# Build toolchain: cross compiler + the tools the Taskfile's check-tools wants
# (go unzip git wget), curl/zip for downloads, plus Node.js for frontend-build.
# Note: `sudo` is required because the Taskfile's download-tflite/-onnxruntime
# helpers shell out to `sudo` when installing libs to system dirs. We run as
# root, so a passthrough sudo is enough.
RUN apt-get update -q && apt-get install -q -y --no-install-recommends \
        curl git wget zip unzip ca-certificates sudo \
        gcc-aarch64-linux-gnu \
    && rm -rf /var/lib/apt/lists/*

SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Task + nfpm. (build.sh can also install nfpm itself, but prebaking is faster.)
RUN sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin
RUN go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
ENV PATH="/root/go/bin:${PATH}"

WORKDIR /src
# Full source (incl. .git) so build.sh can derive the version from tags.
COPY . .

ENV PUPPETEER_SKIP_DOWNLOAD=true
RUN ARCH=${ARCH} VERSION=${VERSION} MAINTAINER=${MAINTAINER} OUTDIR=/out packaging/deb/build.sh

# Export stage: buildx --output type=local copies just the .deb out.
FROM scratch AS export
COPY --from=builder /out/ /
