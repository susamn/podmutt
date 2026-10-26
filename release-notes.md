## Highlights

- **Playback URL Inspector (`I`)** — View stored URLs for the active episode and feed at any time. Press `I` to open a read-only, scrollable popup displaying RSS, audio, artwork, and chapter links. Scroll with `j`/`k`, arrows, or `PgUp`/`PgDn`, and dismiss with `Esc` or `I`.
- **Themed Popups & Consistent Labels** — Unified visual styling across Settings, Help, URL inspection, confirmations, and error dialogs using palette accent and cyan highlights for section headers and field labels with muted shortcut hints.
- **Sanitized Modal Rendering** — Strips ANSI escape sequences from remote metadata and URLs so long addresses wrap cleanly without terminal formatting glitches.
- **Darwin Bridge Test Coverage & Gate** — Combined real-mpv and native macOS media bridge test profiles into `native-verify`, enforcing the 95% statement coverage threshold alongside regression coverage for Apple rating recovery and IPC error replies.

Audio streaming requires `mpv`.
