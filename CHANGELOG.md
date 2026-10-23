# Changelog

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
- v1.3.0 was published under an explicit owner-approved verification waiver. Native macOS runtime/media controls, physical media-key arbitration, real Kitty display behavior, native backup/restore, and clean install/upgrade/uninstall checks remain unverified; see [`docs/validation-v1.md`](docs/validation-v1.md).
