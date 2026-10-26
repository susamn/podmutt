# Linux Distribution Package Contract & Distribution Plan

Status: implemented for Milestone 1.4.0 (Issues #48, #45, #43, #44, #47, #46).

This document defines the canonical specification for `podmutt` standalone Linux binary packages (`.deb` and `.rpm`), the release payload manifest, dependency contracts, and installation lifecycle guarantees.

---

## 1. Distribution Scope & Targets

### Target Distributions
- **Debian Stable**: Primary target distribution family (Debian 12 "Bookworm", Ubuntu 22.04/24.04 LTS compatible). Package format: Debian binary package (`.deb`).
- **Fedora (Current)**: Primary RPM target distribution family (Fedora 40/41, Enterprise Linux / RHEL / CentOS Stream compatible). Package format: Red Hat package (`.rpm`).

### Supported Architectures
- **amd64** (`x86_64`): Tier 1. Built and smoke-tested in automated CI container workflows.
- **arm64** (`aarch64`): Tier 1 cross-compilation. Built and package-inspected in CI; native runtime execution verified on physical/aarch64 runners where available.

### Out of Scope for Milestone 1.4.0
- Signed APT/DNF repositories (e.g., hosted Debian archive / Copr / PPA). Packages are distributed as standalone, immutable GitHub Release assets.
- Arch Linux AUR, Nix flakes/packages, and official distribution main-archive packaging remain independent downstream channels.

---

## 2. Version Normalization & Package Naming

### Version Normalization
Upstream Git tags follow semantic versioning with a leading `v` (e.g. `v1.4.0`, `v1.4.0-rc.1`). Packaging systems require tag normalization:

| Tag Type | Git Tag | Package Version (DEB) | Package Version (RPM) | Release / Revision |
| --- | --- | --- | --- | --- |
| Stable Release | `v1.4.0` | `1.4.0` | `1.4.0` | `1` |
| Pre-release / RC | `v1.4.0-rc.1` | `1.4.0~rc1` | `1.4.0~rc1` | `1` |
| Snapshot / Dev | `v1.4.0-next` | `1.4.0~next` | `1.4.0~next` | `1` |

- The leading `v` is stripped from the version string.
- Hyphenated pre-release identifiers are translated to `~` to ensure correct package manager version ordering (where `1.4.0~rc1 < 1.4.0`).

### Package Architecture Naming
- **Debian**: `amd64`, `arm64`
- **RPM**: `x86_64`, `aarch64`

### Package File Naming Convention
- **DEB**: `podmutt_<version>-1_<arch>.deb` (or `podmutt_<version>_<arch>.deb`)
- **RPM**: `podmutt-<version>-1.<arch>.rpm` (or `podmutt-<version>.<arch>.rpm`)

---

## 3. Canonical Installation Manifest & Payload

All Linux packaging (DEB, RPM, Makefile `install-linux`, and future AUR packages) must strictly conform to this shared installation manifest:

| Source File | Installed Path | Permissions | Ownership | Description |
| --- | --- | --- | --- | --- |
| `bin/podmutt` | `/usr/bin/podmutt` | `0755` | `root:root` | Main static application binary (Linux ELF) |
| `packaging/linux/podmutt.desktop` | `/usr/share/applications/podmutt.desktop` | `0644` | `root:root` | XDG desktop application entry |
| `assets/logo.svg` | `/usr/share/icons/hicolor/scalable/apps/podmutt.svg` | `0644` | `root:root` | Scalable vector application icon |
| `assets/logo.png` | `/usr/share/icons/hicolor/512x512/apps/podmutt.png` | `0644` | `root:root` | High-resolution raster application icon |
| `LICENSE` | `/usr/share/doc/podmutt/copyright` | `0644` | `root:root` | MIT license notice (Debian standard path) |
| `README.md` | `/usr/share/doc/podmutt/README.md` | `0644` | `root:root` | Upstream documentation |

### Strict Exclusions
- **No Darwin Artifacts**: No macOS binaries, `.icns` files, or `Info.plist` files shall be packaged in Linux releases.
- **No Go Runtime / Compiler Dependencies**: The binary is statically compiled (`CGO_ENABLED=0`).
- **No Dynamic SQLite Dependency**: Embedded SQLite is compiled into the binary via modernc pure-Go SQLite; no `libsqlite3.so` dependency is declared.

---

## 4. Package Metadata Contract

Package metadata must match upstream project definitions:

- **Package Name**: `podmutt`
- **Maintainer / Vendor**: `Supratim Samanta <supratim.iee23.ju2005@gmail.com>`
- **Homepage**: `https://github.com/susamn/podmutt`
- **License**: `MIT`
- **Debian Section**: `sound`
- **Debian Priority**: `optional`
- **RPM Group**: `Applications/Multimedia`
- **Summary**: `Modern, keyboard-driven podcast client (TUI)`
- **Description**:
  ```text
  podmutt is a keyboard-driven terminal podcast client for Linux inspired
  by lazygit and mpdtui. It features local-first SQLite library storage,
  instant Top 100 directory browsing, audio streaming and playback via mpv,
  responsive layouts, and Linux desktop media key integration via MPRIS.
  ```

---

## 5. Dependency Contracts

### Runtime Dependencies
- **`mpv`**: Required for audio playback. Declared without a strict minimum version bound so that distributions may satisfy it with their supported `mpv` package (e.g. `mpv >= 0.30` or unversioned `mpv`). Audio streaming interacts via standard JSON IPC over private domain sockets.
  - Debian: `Depends: mpv, ca-certificates`
  - RPM: `Requires: mpv, ca-certificates`
- **`ca-certificates`**: Required for secure TLS connections to podcast feeds and remote directories.
- **Terminal & Font**: Standard terminal emulator supporting UTF-8 and ANSI colors. Nerd Fonts are optional and configurable; default glyphs use standard Unicode.

### Packaging Rule: No Bundled Media Player
`mpv` is an external runtime dependency, **never bundled** inside the `podmutt` package. This honors distribution shared library policies and security updates.

---

## 6. Lifecycle & Non-Destructive Invariants

To guarantee system stability, user safety, and adherence to Linux distribution standards:

1. **No System Daemons / Services**:
   - The package must NOT install `systemd` unit files, init scripts, or background cron jobs.
   - `podmutt` runs solely as an interactive foreground user process or CLI control invocation.

2. **No Dedicated System Users**:
   - The package must NOT create a `podmutt` system user or group (`useradd`/`groupadd`). It runs with the privileges of the invoking user.

3. **No Network Access at Post-Install**:
   - Post-installation (`postinst` / `%post`) must not invoke `curl`, `wget`, or any network operations.
   - No external assets or binaries are downloaded during install or uninstall.

4. **User Profile Preservation**:
   - User profile data, downloaded feeds, cache, and configuration reside in `$XDG_CONFIG_HOME/podmutt` (or `~/.config/podmutt`) and `$XDG_DATA_HOME/podmutt`.
   - Package upgrades, uninstalls (`apt remove`, `dnf remove`), and complete package purges (`apt purge`) MUST NOT delete or alter user directories or files in user home directories.
   - Profile ownership and retention remain strictly with the user.

---

## 7. Verification Gates

Every built package artifact must pass deterministic smoke validation before publication:
1. **Inspection**:
   - Verify archive structure using `dpkg-deb -c` / `dpkg-deb -I` (for DEB) and `rpm -qlp` / `rpm -qip` (for RPM).
   - Ensure all payload paths match Section 3 exactly.
   - Ensure file permissions: binaries `0755`, desktop/icons/docs `0644`.
   - Ensure absence of Darwin binaries or unreferenced assets.
2. **Execution Smoke Test**:
   - Execute the packaged binary directly from an isolated container / path:
     - `/usr/bin/podmutt --version`
     - `/usr/bin/podmutt --help`
3. **Container Lifecycle**:
   - Fresh install via `apt install ./podmutt_*.deb` and `dnf install ./podmutt-*.rpm`.
   - Upgrade test over a previous version.
   - Clean uninstall test ensuring clean exit and zero lingering background processes.
