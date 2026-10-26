# Changelog

## [1.4.0] - 2026-10-26

### Added

- **Native Linux DEB and RPM Packages:** standalone binary packages for Debian/Ubuntu (`.deb`) and Fedora/RHEL (`.rpm`) supporting both `amd64` and `arm64` architectures.
- **Canonical Installation Manifest:** standardized `/usr` hierarchy with desktop entry (`/usr/share/applications/podmutt.desktop`), scalable and 512x512 icons, copyright notice, and documentation.
- **Package Verification Suite:** deterministic inspection script `packaging/linux/verify-packages.sh` (`make package-verify`) verifying metadata, payload paths, static ELF linking, and runtime smoke execution.
- **Container Lifecycle Testing:** containerized test suite `packaging/linux/test-install-lifecycle.sh` (`make package-lifecycle`) verifying clean `apt`/`dnf` installs, non-root execution, upgrade resilience, and user profile data retention.

### Changed

- **Release Automation Hardening:** pinned GoReleaser to `v2.18.3`, aligned Go selection across workflows with `go.mod`, configured immutable release assets (`mode: keep-existing`), and integrated `mpv-test` and `mpris-test` gates.
- **Decoupled Downstream Publishing:** separated core immutable release publishing from downstream Homebrew tap updates, preventing downstream network or token failures from blocking the primary release.
- **Documentation:** updated `README.md` to document manual `.deb` and `.rpm` installations, clarified channel boundaries, and corrected Homebrew `--cask` documentation to designate it as macOS-only.

## [1.3.1] - 2026-10-26

### Added

- **Playback URL inspection:** press `I` to open a scrollable, read-only URL modal for the active playback (feed RSS, feed artwork, audio, episode artwork, and chapters links). Scroll with `j`/`k`, arrows, or `PgUp`/`PgDn`; close with `Esc` or `I`.
- **Themed popup typography:** unified structural labels, sections, and hints across all dialogs (Settings, Help, URL modal, feedback/error messages, and input/confirmation dialogs) styled with active theme colors.

### Changed

- **Sanitized modal rendering:** remote titles, URLs, and error summaries are scrubbed of terminal escape sequences before rendering, preventing styling leaks while keeping structural labels highlighted.
- **Mac bridge test verification:** bundled native media tests and real-mpv playback coverage are incorporated into the Darwin verification gate (`native-verify`) to enforce the aggregate 95% statement coverage threshold.

## [1.3.0] - 2026-10-23

### Added

- **Favorites:** persistent Favorites pane (`5`), independent of the playback queue. Mark a queued episode from its actions menu (`m`); remove it from Favorites with `d`/Delete.
- **Chapters:** show cached chapter indicators in episode and queue lists. Press `C` to browse and jump within the current playback; chapter jumps preserve paused/playing state. Supports linked Podcasting 2.0 chapters, inline Podlove chapters, and embedded mpv chapters.
- **Volume controls:** press `=` / `-` anywhere outside text input or dialogs to adjust playback volume by 5%, bounded to 0–130%. Starts at 100% each launch; changes are not persisted.
- **Discover restoration:** Apple Discover results and view selection return immediately from saved snapshots, then refresh asynchronously. A failed refresh retains cached Apple results and offers retry.
- **Actionable feedback and shared dialogs:** clearer recovery guidance for playback, Discover, settings, and input failures; consistent Help, Settings, confirmation, and input-dialog layouts.
- **Queue removal recovery:** confirm removal of the selected queued episode, including current playback; undo the most recent removal with `u`.

### Changed

- **Publication dates:** episodes published within the last week use relative labels such as “Today” and “3 days back”; older dates remain calendar dates.
- **Compact layout:** Player appears below the active pane in compact layouts. The wide layout is unchanged. Player is pane `6`; Favorites is pane `5`.
- **Keyboard help:** documents current pane numbers, global playback controls, and context-owned input/modal keys.

### Notes

- Podcast Index search results are not durably cached; only provider navigation state is saved, in accordance with its provider policy.
- Audio streaming requires `mpv`. Volume resets to 100% when podmutt restarts.
