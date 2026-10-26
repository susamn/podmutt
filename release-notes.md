## Highlights

- **Native Linux DEB & RPM Packages** — First-class standalone `.deb` and `.rpm` binary release assets for both `amd64` and `arm64` Linux distributions, adhering to the canonical filesystem hierarchy (`/usr/bin/podmutt`, desktop entry, scalable and 512x512 icons, copyright, and documentation).
- **Package Manager Dependency Resolution** — Declares external runtime dependencies on `mpv` and `ca-certificates`, automatically resolved through distribution package managers (`apt` on Debian/Ubuntu, `dnf` on Fedora/RHEL) without bundling `mpv`.
- **Non-Destructive Lifecycle Guarantees** — No systemd services, daemons, or system users are created. Upgrading, removing, or purging packages (`apt purge`, `dnf remove`) strictly preserves user configurations, preferences, catalog databases, and listening progress in `~/.config/podmutt` and `~/.local/state/podmutt`.
- **Package Inspection & Lifecycle Testing** — Added package payload and static linking verification (`make package-verify`) and containerized lifecycle test suites (`make package-lifecycle`) verifying clean root installation, non-root execution, upgrade safety, and user profile retention.
- **Hardened Release Pipeline** — Pinned GoReleaser tooling, aligned Go toolchains with `go.mod`, enforced immutable releases (`keep-existing`), integrated `mpv` and `mpris` test gates, and decoupled downstream Homebrew tap synchronization from core release publication.

Audio streaming requires `mpv`.
