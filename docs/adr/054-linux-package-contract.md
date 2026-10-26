# ADR 054: Linux DEB/RPM package contract and canonical install manifest

Status: accepted for Milestone 1.4.0.

## Context
podmutt previously offered cross-compiled tarballs and a macOS Homebrew formula. Linux users on Debian/Ubuntu and Fedora families require native distribution packages (`.deb` and `.rpm`) with proper metadata, desktop file integration, icons, and package manager dependency resolution for `mpv` and CA certificates.

## Decision
1. **Target Families**:
   - Debian stable (and Ubuntu LTS derivatives) via `.deb`.
   - Fedora (and Enterprise Linux / RHEL derivatives) via `.rpm`.
   - Architectures: `amd64` (`x86_64`) and `arm64` (`aarch64`).
2. **Canonical Payload Layout**:
   - `/usr/bin/podmutt` (mode 0755, root:root)
   - `/usr/share/applications/podmutt.desktop` (mode 0644)
   - `/usr/share/icons/hicolor/scalable/apps/podmutt.svg` (mode 0644)
   - `/usr/share/icons/hicolor/512x512/apps/podmutt.png` (mode 0644)
   - `/usr/share/doc/podmutt/copyright` (mode 0644, containing LICENSE)
   - `/usr/share/doc/podmutt/README.md` (mode 0644)
3. **Dependency Contract**:
   - Declare runtime dependencies on `mpv` and `ca-certificates` without arbitrary minimum version bounds.
   - Do NOT bundle `mpv` inside podmutt packages.
   - Pure-Go embedded SQLite and static linking mean zero dynamic C library dependencies.
4. **Lifecycle Guarantees**:
   - No background systemd units or system services.
   - No system user or group creation.
   - No post-install network calls.
   - User profile data in `~/.config/podmutt` and `~/.local/share/podmutt` is strictly preserved on upgrade, removal, and purge.

Detailed specifications and architecture matrices are maintained in `docs/distribution-plan.md`.
