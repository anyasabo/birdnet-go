# TODO: Native `.deb` install for Raspberry Pi (Docker-free), fork-maintained

**Status:** Implemented + locally proven (build + emulated install). Not yet run
on real Pi hardware; not committed to upstream; no apt repo yet.
**Owner:** unassigned
**Created:** 2026-06-06

## Goal

Give Raspberry Pi users a lightweight install that does **not** require pulling a
large Docker image onto an embedded device. Ship a native `.deb` that runs the
`birdnet-go` binary directly under systemd, with the audio tooling resolved by
apt. The whole thing is **fork-maintained**: upstream may never take it, so it
must keep working on our fork and survive re-syncing onto `upstream/main`.

**Hard constraint that shaped everything:** all packaging lives in *net-new
files* only. We never edit upstream-owned files (`Taskfile.yml`,
`.github/workflows/release-build.yml`, `Dockerfile`, `.dockerignore`,
`doc/wiki/installation.md`). Merge conflicts only happen on files both sides
touch, so additive-only ⇒ `git rebase upstream/main` stays clean.

## Current state: what works

Everything below is **done and verified locally** (macOS + Docker Desktop):

- **Build (cross-compiled, no emulation):** `packaging/deb/build-docker.sh`
  produced `birdnet-go_0.6.4+fork.20260607.gb194ecc5_arm64.deb` (94 MB).
  Verified the binary + both libs are genuine `ELF aarch64`.
- **Emulated install smoke test passed:** `packaging/deb/test-install.sh` ran
  `apt install ./pkg.deb` in an arm64 `debian:trixie-slim` container. Confirmed:
  apt resolved Depends, `postinst` created the `birdnet-go` user + ran
  `ldconfig`, the vendored libs registered with the linker, and the arm64 binary
  exec'd cleanly (`--help`) — i.e. its `.so` deps load and CGO links.
- nfpm package structure verified: correct version, `Architecture: arm64`,
  `Depends: ffmpeg, sox, libsox-fmt-mp3, libasound2`, `config.yaml` as a
  `noreplace` conffile, all three maintainer scripts embedded.

## What was investigated (so a future agent doesn't repeat it)

- **The binary is self-contained for the common case.** Frontend is embedded
  (`frontend/embed.go`, `//go:embed all:dist`) and the BirdNET v2.4 models are
  embedded (`internal/classifier/models_embedded.go`). So no model/asset
  download on first run for the default model.
- **Release CI already bundles the exact artifacts we need.**
  `.github/workflows/release-build.yml:115-122` ships `birdnet-go` +
  `libtensorflowlite_c.so` + `libonnxruntime.so` per platform. The `.deb` is
  essentially "that tarball, packaged properly."
- **Only true runtime deps** beyond the two vendored `.so`s are `ffmpeg` (RTSP +
  non-WAV export + live stream), `sox` + `libsox-fmt-mp3` (spectrogram render),
  `libasound2` (mic capture). The maintainer confirmed these are **required, not
  optional**. The Dockerfile's runtime stage (`debian:trixie-slim`) pulls the
  same packages (`Dockerfile:97-103`), so depending on distro packages is NOT a
  regression vs Docker.
- **Distro freshness downside (minor):** RPi OS = Debian stable (frozen ~2yr,
  security backports only). For these deps it barely matters (libasound2 stable
  ABI; sox upstream is dead; ffmpeg is the only one where version could matter).
  The one real gap: the Docker image is built on trixie (Debian 13) while a Pi
  host is bookworm (Debian 12), so the container can run a newer ffmpeg than apt
  gives natively. Escape hatch if it ever bites: vendor a static ffmpeg the same
  way we vendor the `.so`s, and drop the `ffmpeg` Depends.
- **Config/data paths:** the binary's default config search includes
  `/etc/birdnet-go/` (`internal/conf/utils.go:116`), and default data paths in
  `internal/conf/config.yaml` are *relative* (`clips/`, `logs/`). So the unit
  sets `WorkingDirectory=/var/lib/birdnet-go` and data lands there. Run command
  is `birdnet-go realtime` (matches `Dockerfile` CMD).
- **Cross-compile, not emulation:** `task linux_arm64` already cross-compiles via
  `gcc-aarch64-linux-gnu` (`Taskfile.yml`, `CROSS_LIB_DIR_ARM64`). So we build
  the arm64 binary inside a *native* linux/amd64 container — full speed. Only the
  install *test* needs qemu (to run arm64 scripts + exec the binary).

## Gotchas already hit and fixed (don't rediscover these)

1. **nfpm `src:` is relative to invocation cwd**, not the config file. `build.sh`
   stages into `packaging/deb/staging/`, so nfpm.yaml `src:` paths use the full
   `packaging/deb/staging/...` prefix.
2. **Container needs `sudo`** — the Taskfile's `download-tflite` /
   `download-onnxruntime` shell out to `sudo` even when writing to writable dirs.
   `packaging/deb/Dockerfile` installs it.
3. **`.git` is dockerignored** (`.dockerignore:4`), so version derivation fails
   inside the build container. Fixed by computing the version on the HOST
   (`packaging/deb/version.sh`) and passing it in as a `--build-arg`. Shared by
   both build scripts so they can't drift.
4. **ONNX is NOT fetched by `task linux_arm64`**, and `task download-onnxruntime`
   keys off the *build-host* arch (wrong for cross builds — that's why upstream
   CI uses a separate action). `build.sh` now downloads the correct *target-arch*
   ONNX directly from upstream's pinned release, reading `ONNXRUNTIME_VERSION`
   from `Taskfile.yml` so it tracks upstream.

## What to do next

1. **Real Pi verification (the one untested path).** Install the `.deb` on actual
   Pi hardware and confirm the **systemd unit** starts and **live mic capture
   works** under the hardened unit: `User=birdnet-go` + `SupplementaryGroups=audio`
   + `ProtectSystem=full` + `PrivateTmp=true`. If capture fails, the likely
   culprits are `/dev/snd` access (group/ProtectSystem) — loosen `ProtectSystem`
   to `true` or check device perms. The emulated test can't cover this (no
   systemd PID 1; postinst's `systemctl` steps are skipped by the
   `[ -d /run/systemd/system ]` guard).
2. **ONNX soname nuance.** ldconfig registered `libonnxruntime.so.1` (the lib's
   internal SONAME) while we ship bare `libonnxruntime.so`. tflite is a hard
   `NEEDED` link and resolves; ONNX is `dlopen`'d lazily for Perch v2 / bat /
   geomodel features. This matches how upstream ships it (bare `.so`), so not a
   regression — but if those ONNX features fail to load on the Pi, ship the
   versioned soname too (e.g. add a `libonnxruntime.so.1` content/symlink).
3. **Commit + push + tag.** Push branch, then `git tag pkg-YYYY.MM.DD` to trigger
   `.github/workflows/fork-deb.yml`, which builds both arches and uploads the
   `.deb`s to *this fork's* GitHub Releases.
4. **(Optional) True `apt upgrade`.** Publish a signed flat apt repo to the fork's
   GitHub Pages (`dpkg-scanpackages` or `aptly`, GPG-signed) + a
   `/etc/apt/sources.list.d/birdnet-go.list` on the Pi. Deferred until the basic
   package is proven on hardware.
5. **(Optional) Docs.** Add a native-binary section to `doc/wiki/installation.md`
   — but that's an upstream-owned file, so only do it in a PR *to upstream*, not
   on the fork (keep the fork additive).

## Resync runbook (keep the fork conflict-free)

```bash
git fetch upstream && git rebase upstream/main   # clean: packaging is all new files
git push origin main
git tag pkg-$(date +%Y.%m.%d) && git push origin --tags
```

The only coupling to upstream is `build.sh`'s `.so` lookup (`LIB_SEARCH`), which
tracks `release-build.yml` *by convention*, not by patching it. If upstream
relocates the libs, the build fails loudly (never a silently broken package); fix
is to update `LIB_SEARCH`.

## Key files (all net-new under `packaging/deb/`)

- `nfpm.yaml` — package definition (contents, Depends, scripts, conffile)
- `build.sh` — orchestrator: calls `task linux_<arch>`, stages, fetches ONNX, runs nfpm
- `build-docker.sh` — local cross-compile build in a container (macOS-friendly)
- `version.sh` — shared host-side version derivation (`<v-tag>+fork.<date>.g<sha>`)
- `Dockerfile` — the cross-compile builder image
- `test-install.sh` — emulated arm64 `apt install` smoke test
- `birdnet-go.service` — systemd unit (runs `birdnet-go realtime`)
- `ld.so.conf`, `scripts/{postinstall,preremove,postremove}.sh`
- `README.md` — user + maintainer runbook
- `.github/workflows/fork-deb.yml` — fork-only CI (net-new, never conflicts)
