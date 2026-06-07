# Fork-maintained `.deb` packaging

A native Debian package for BirdNET-Go that runs the binary directly under
systemd — **no Docker**, no large image pull. Built for Raspberry Pi (arm64)
and x86-64 (amd64).

This exists entirely as net-new files so it survives upstream resyncs without
merge conflicts (see "Resyncing the fork" below). If upstream ever adopts native
packaging, delete `.github/workflows/fork-deb.yml` and point at theirs.

## What the package does

| Path                                     | Purpose                                           |
| ---------------------------------------- | ------------------------------------------------- |
| `/usr/bin/birdnet-go`                    | Binary (frontend + BirdNET v2.4 models embedded)  |
| `/usr/lib/birdnet-go/*.so`               | Vendored TFLite + ONNX Runtime libraries          |
| `/etc/ld.so.conf.d/birdnet-go.conf`      | Registers the lib dir with the linker             |
| `/etc/birdnet-go/config.yaml`            | Config (conffile — your edits survive upgrades)   |
| `/lib/systemd/system/birdnet-go.service` | systemd unit (`birdnet-go realtime`)              |
| `/var/lib/birdnet-go/`                   | Data dir (db, clips, logs), owned by service user |

Runtime deps (`ffmpeg`, `sox`, `libsox-fmt-mp3`, `libasound2`) are declared as
`Depends:` and resolved by apt — these are required, not optional.

The `postinstall` script creates a `birdnet-go` system user (added to `audio`
for mic capture), runs `ldconfig`, and enables + starts the service.

## Install / update on the Pi

```bash
# Grab the latest fork release asset (deps auto-resolved by the leading ./)
curl -fsSL https://github.com/<you>/birdnet-go/releases/latest/download/birdnet-go_<ver>_arm64.deb -o bn.deb
sudo apt install ./bn.deb
```

Updating is the same command with a newer `.deb`; dpkg treats the higher
`+fork.<date>.<sha>` version as an upgrade and preserves `config.yaml` and
`/var/lib/birdnet-go`. Manage it with standard systemd:

```bash
systemctl status birdnet-go
journalctl -u birdnet-go -f
```

Remove with `sudo apt remove birdnet-go` (keeps data) or `sudo apt purge
birdnet-go` (also drops the service user; data dir is left for safety).

## Building

### Locally on any host (incl. macOS) — Docker, no emulation

```bash
packaging/deb/build-docker.sh              # arm64 (Raspberry Pi), default
ARCH=amd64 packaging/deb/build-docker.sh   # amd64
# -> dist/deb/birdnet-go_<version>_<arch>.deb
```

The binary is **cross-compiled** inside a native `linux/amd64` container
(`packaging/deb/Dockerfile`) using `gcc-aarch64-linux-gnu` — the same toolchain
upstream's `task linux_arm64` uses. No qemu, so an arm64 package builds at full
speed. The version is computed on the host (`.git` is dockerignored) and passed
in as a build-arg.

### Natively on Linux / on the Pi — no Docker

```bash
packaging/deb/build.sh              # arm64
ARCH=amd64 packaging/deb/build.sh   # amd64
```

Needs Go + Task (nfpm is auto-installed via `go install` if missing). This is
also what CI runs.

### CI / publishing

`.github/workflows/fork-deb.yml` runs `build.sh` for both arches and, on a
`pkg-*` tag, uploads them to this fork's Releases:

```bash
git tag pkg-$(date +%Y.%m.%d) && git push origin pkg-$(date +%Y.%m.%d)
```

### How it stays decoupled

`build.sh` only *calls* upstream's `task linux_<arch>` target — it never edits
the Taskfile. It owns two things: locating `libtensorflowlite_c.so` after the
build (via `LIB_SEARCH`), and fetching the correct target-arch ONNX Runtime
directly from upstream's pinned release (`ONNXRUNTIME_VERSION` is read from the
Taskfile so it tracks upstream). If the lib lookup ever fails it's a loud build
error — never a silently broken package — and the fix is to update `LIB_SEARCH`
to match wherever `release-build.yml` copies the libs from.

## Versioning

`<upstream-tag>+fork.<utc-date>.g<short-sha>`, e.g. `0.6.4+fork.20260606.gb194ecc5`.
Derived from the latest upstream `v*` tag (the floating `nightly-*` tag is
ignored). Every rebuild after a resync produces a strictly higher version.

## Resyncing the fork

```bash
git fetch upstream
git rebase upstream/main        # clean: packaging is all net-new files
git push origin main
git tag pkg-$(date +%Y.%m.%d)   # triggers a fresh package build
git push origin --tags
```

Because nothing here edits an upstream-owned file, the rebase never conflicts on
packaging. The only coupling is `build.sh`'s `.so` lookup, which tracks
`release-build.yml` by convention, not by patching it.

## Optional: true `apt upgrade` via an apt repo

The releases-asset flow above is the simplest path. For
`apt update && apt upgrade birdnet-go`, publish a signed flat apt repo to this
fork's GitHub Pages (`dpkg-scanpackages` or `aptly`, GPG-signed) and drop a
`/etc/apt/sources.list.d/birdnet-go.list` on the Pi. Deferred until the basic
package is proven.
