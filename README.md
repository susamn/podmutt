# podmutt

A Linux/macOS podcast TUI inspired by lazygit and mpdtui. Planned stack: Go,
Bubble Tea, SQLite, and mpv. Licensed under [MIT](LICENSE).

**Status:** cached podcast browsing, feed refresh, selected playback, queue editing,
Apple discovery/subscription previews, library search, and running-instance
controls are available.
`--help` and `--version` work without creating profile files.

## Development

Requires Go 1.27.2+, Make, and a C compiler for race tests. Runtime playback will
require a separately installed mpv; no Go toolchain is needed for release users.

```sh
make check       # formatting, module consistency, vet, tests, >=90% coverage
make race        # native race-detector tests
make mpv-test    # real-mpv lifecycle/playback tests, null audio; requires mpv
make mpris-test  # private D-Bus protocol/lifecycle tests; requires dbus-daemon
make build       # bin/podmutt
make dist        # Linux/macOS amd64/arm64 archives in dist/
```

`make fmt` formats source. Coverage reports are written to `coverage/`.
`make clean` removes generated build and coverage artifacts. On memory-constrained
machines use `GOGC=20 make race`; CI uses this setting for SQLite compilation.
Cross-build checks compile all application packages, not just the CLI bootstrap.

## GitLab workflow

- Use short-lived branches and small merge requests into `main`.
- Branch pushes run verification, race tests, and a native Linux build.
- Once a merge request is open, its pipeline replaces the branch push pipeline.
- Default-branch and tag pipelines also produce four cross-built archives.
- Coverage is an aggregate first-party statement gate, including command code;
  the threshold is 90%. Generated/vendor code is not currently present.
- CI retains coverage reports and Linux builds for 14 days; cross-build archives
  for 30 days. These are CI artifacts, not permanent published releases.
- Cross-building does not prove native macOS runtime behavior. Native macOS
  CGO/media-key verification is explicitly pending; see the native checklist.
  Signing/notarization and package publication remain release work.

`main` is protected: direct pushes and force pushes are disabled, and Maintainers
merge reviewed MRs only after a successful pipeline. Skipped pipelines do not
satisfy the merge gate. Online project runners accept untagged Linux jobs; shared
runners are not required. These settings live in GitLab, outside `.gitlab-ci.yml`.
Native macOS tests will additionally need a macOS runner.

Before requesting review, run `make check` and `make race`, then push the branch
and open an MR. Check its pipeline and downloadable coverage/build artifacts.
`make coverage COVERAGE_MIN=101` should fail and can be used to verify rejection
without changing source or weakening the normal 90% threshold.

## Design

- [V1 scope](docs/spec-v1.md)
- [Foreground lifecycle ADR](docs/adr/001-foreground-lifecycle.md)
- [SQLite storage ADR](docs/adr/003-sqlite-storage.md)
- [mpv ownership ADR](docs/adr/004-mpv-ownership.md)
- [Responsive TUI ADR](docs/adr/005-tui-shell.md)

## Linux media controls

The foreground app exposes MPRIS on an existing Linux session D-Bus. Desktop
Play/Pause, Stop, and Next use the same playback actor as keyboard/CLI controls.
Next is enabled whenever a pending episode exists; Stop preserves resume
progress. Seeking, Previous, URI opening, and volume/rate/repeat/shuffle changes
are unsupported. A missing/broken bus is nonfatal and never launches a daemon.
macOS has a public MediaPlayer/AppKit CGO bridge prototype in #16, not yet
verified on a native host. Go 1.27 requires **macOS 13+**. `make native-verify`
runs SDK preflight, tests, the actual Objective-C/CGO handler check and real-mpv
null-audio lifecycle tests. `make native-macos` creates an ad-hoc signed bundle;
launch `bin/podmutt.app/Contents/MacOS/podmutt` from a terminal. `make build/dist`
CGO=0 Darwin executables deliberately omit native keys and display a warning.
See [native checks/build implications](docs/macos-media-check.md) and
[ADR 016](docs/adr/016-native-macos-media.md). Native compilation, physical key
behavior and release signing/notarization must not be inferred from Linux CI.

See the [short ADR](docs/adr/010-linux-mpris.md) and
[desktop smoke checklist](docs/linux-media-check.md). Physical desktop media-key
verification remains a separate manual check, not a claim made by cross-builds.

## Recovery and validation

See [backup/restore and migration safety](docs/recovery.md) before upgrading or
repairing a profile. Never copy just a live WAL-mode database or delete its WAL
to repair startup. `make recovery-test` runs bounded local workflow, abrupt-death,
snapshot/restore and rollback fixtures; CI requires that gate. The
[v1 validation ledger](docs/validation-v1.md) explicitly tracks outstanding native
macOS, real Kitty and physical media-key checks. Green cross-builds are not a
substitute for those checks.

## Configuration and profile

The TUI loads configuration and holds exclusive profile ownership until exit.
An absent `config.toml` uses these defaults:

```toml
refresh_minutes = 30
mpv_path = "mpv"
icons = "unicode" # default; "nerd" (Nerd Font Mono) or "plain" (text labels)
telemetry = false
```

Refresh intervals must be positive whole minutes. `mpv_path` is a nonempty
executable name or path. Telemetry is unsupported; enabling it is rejected.
Icons use standard Unicode by default; no patched font is required. Set
`icons = "nerd"` only with a Nerd Font Mono terminal font, or `icons = "plain"`
for readable text without icon prefixes. Labels always remain; plain icon mode
retains the existing TUI borders/progress bar. See [icon policy](docs/adr/025-icon-vocabulary.md).

Malformed TOML and unknown keys are errors. Configuration files are never
rewritten automatically.

Linux configuration lives at `~/.config/podmutt/config.toml`, state at
`~/.local/state/podmutt`; absolute XDG overrides are honored. Runtime files use
`$XDG_RUNTIME_DIR/podmutt` or a private subdirectory of state. macOS uses
`~/Library/Application Support/podmutt/config.toml` with state beside it and a
private `run` directory. See [profile paths and ownership](docs/adr/002-profile-paths.md).

## Shell controls

`j/k` or arrows select; `g/G` go first/last. Outside Player, `h/l` switch panes;
Tab/Shift+Tab or `1-5` switch panes everywhere. `/` filters the focused list; Enter applies, Esc cancels, Ctrl+U
clears input. `?` opens help; `q` or Ctrl+C quits. Printable keys stay literal
while typing a filter.

With **Details (4)** focused, `j/k` or arrows scroll one line, Ctrl-d/Ctrl-u
scroll half a page, `g/G` or Home/End reach the top/bottom, and PgUp/PgDn scroll
a full page. These keys do not change episode selection or playback.

The wide grid keeps Podcasts/Episodes on the left and Queue above the bottom-right
cell. That cell is split evenly: **Details above, Player below**. Long descriptions
scroll with PgUp/PgDn inside their half; they never push the player/controls down.

Press **5** for Player. **h/l seek backward/forward 15 seconds** after an episode
loads, preserving pause state and resume position. Seek hints appear when space
permits; Tab always leaves Player. Native macOS builds expose matching 15-second
Control Center skip commands; absolute system scrubbing remains disabled. Linux
TUI seeking works independently of MPRIS (which still advertises no seek support).
See [seeking policy](docs/adr/021-shared-seeking.md).

Left/Right chooses **Play/Pause, Stop, Next**; Enter
activates. Space/s/n still work as shortcuts. The player shows current title,
status, real-time elapsed/total/remaining and a fractional read-only progress bar;
unavailable buttons are dimmed. The presentation clock updates at 4Hz without
per-frame IPC/database work, freezes paused/stopped, and limits interpolation to
two seconds if actual samples stall. Unknown duration stays unknown. Queue-only
updates preserve the clock; real samples/seeks correct it. See
[progress policy](docs/adr/022-realtime-player-progress.md).
Playback errors remain visible independently of the selected description.
When a tall wide Player pane has spare room, a labelled **Decorative** pattern
scrolls during playback, freezes on pause and clears on stop. It is not an audio
spectrum; compact/crowded layouts hide it and stop its timer.

Below 100 columns or 24 rows, lists stack above a persistent player. Queue/Details
replace the middle list when focused; Esc returns to the last visible list.
Very short terminals show one library pane plus controls. Below 30×12, resize
guidance replaces the panes. See [fixed player layout](docs/adr/014-fixed-player-pane.md).

`N` adds a public RSS/Atom feed or podcast website URL. Enter on a show opens its episodes; `v`
switches Show/New (uncompleted subscribed episodes), and `o` cycles newest,
oldest, and title ordering. `u` toggles the selected show's subscription; `c`
toggles completion in the episode pane. `R` refreshes manually. Cached content
opens before background refresh. My subscriptions lists subscribed shows only;
unsubscribing retains cached episodes/history for resubscription. `[+]` indicates
subscription, `[x]` indicates completion. PgUp/PgDn scroll
details. See [library UI policy](docs/adr/009-library-ui.md).

Space plays the selected Episodes/Queue item, or toggles it if current; elsewhere
it toggles current playback. Enter opens Details in Episodes; **Queue Enter plays
and promotes the selected row**. `s` stops and `n` advances to the first pending
episode. The Queue shows current playback plus pending episodes with show/title,
state and resume position. Playing a different episode pushes interrupted playback
behind it; completion removes the row but keeps history. Restart restores stopped.
`a` enqueues one item; `A` enqueues visible Episodes in display order. In Queue,
`j/k` select, `J/K` move pending items, `d` removes a pending item, and `D` clears
pending items while keeping current playback. Removal/clear require explicit `y`;
Esc/n cancels. A changed queue invalidates clear confirmation. Queue edits do not
change current playback/history. Status shows position, optional duration, and
persistent playback failures. See [player/queue policy](docs/adr/011-player-queue-ui.md).

Subscribed shows automatically add their latest cached episode at startup,
on subscription and after successful refresh (including 304). Arrivals append
behind current/pending playback without autoplay or historical backlog. Completed,
current and already-seen heads are not re-added after removal/restart. Ad-hoc shows
never auto-enqueue. Unsubscribe keeps queued work/history; resubscribe keeps these
receipts. See [arrival policy](docs/adr/020-subscribed-latest-autoqueue.md).
Schema migration 2 preserves listening state; older binaries cannot open it—use a
consistent backup before upgrading, not a forced downgrade.

## Discover and subscribe

An empty profile opens **Discover**, automatically loading Apple's **US top-25**
shows. With existing subscriptions, startup stays local-first in **My subscriptions**.

1. Press `b` to switch My subscriptions / Apple Discover.
2. Select an Apple show and press Enter to preview its RSS episodes. Nothing is
   subscribed, played, or downloaded as audio at this stage.
3. Press **Space to play without subscribing**, or `a`/`A` to enqueue the selected/
   visible preview episodes. Only explicit selections are cached; show subscription
   and feed refresh validators are unchanged. Space toggles if that episode is
   already current, just like stored episodes; Player Space toggles current audio.
4. Press `u` only if you want to subscribe. The complete preview is then cached,
   preserving ad-hoc playback identity/history. Browsing/preview alone stays read-only.

`f` opens search in the current view. Tab in the dialog switches **Library / Apple**;
Enter submits, Esc cancels, Ctrl+U clears. Library searches subscribed show names
and cached episode titles/descriptions, including completed history. Apple searches
shows only—not a global episode index. `/` still filters only the focused list.
Esc clears library search or leaves a preview. An empty Apple query restores the
chart; `R` retries/reloads directory results. Quota waits can take a few seconds.

Apple access is public/no-key, not an import of Apple account subscriptions.
Network failure leaves your library usable with `b`; errors never erase it.
Artwork renders asynchronously beside Details, using episode art then show-art
fallback. Kitty requires a positive graphics probe outside tmux/screen; other
terminals use image-derived ASCII. Compact/modal layouts hide art, and failure
shows a placeholder without affecting playback. Real Kitty verification remains
[pending](docs/artwork-check.md). See
[discovery policy and bounds](docs/adr/012-show-discovery.md).

## Running-instance commands

`podmutt play`, `pause`, `stop`, `next`, and `status` contact the running TUI.
Only `status` prints JSON. They never start a player or create profile files when
the TUI is absent. Exit codes: 0 success, 1 failure, 2 usage, 3 not running.
See [private controls](docs/adr/008-running-instance-control.md).

## Storage

The foreground session opens `podcasts.db` for subscriptions, episodes, history,
current episode, and queue. It restores playback without autoplay.
Do not copy only an open database file for backup; SQLite WAL sidecars may contain
recent writes. Follow the [consistent backup/restore instructions](docs/recovery.md);
backup does not mean copying a live DB file or automatically downgrading schemas.

## Player adapter

The playback service starts mpv lazily for the current/queued episodes. It uses
a private inherited socketpair, separate from the application control socket. Missing
mpv produces installation guidance. User mpv configuration/scripts are disabled.
Run `make mpv-test MPV=/path/to/mpv` to verify resume, pause/play/stop, EOF versus
errors, graceful shutdown, and exit after abrupt owner death. No audio device or
network podcast is needed. Linux CI runs these checks; native macOS validation
still needs a macOS host.
