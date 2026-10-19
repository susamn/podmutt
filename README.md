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

Optional [SonarQube Community Build analysis](docs/sonarqube.md) runs only on
`main` after successful verification/builds, using the LAN Docker runners and
existing coverage artifact. Configure protected Sonar variables to enable it;
MRs never upload analysis to the imported project's main branch.

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
player_decoration = false # opt in to decorative animation in spacious layouts
telemetry = false
theme = "colors" # default on Linux; "solarized-dark", "catppuccin", or custom
```

Refresh intervals must be positive whole minutes. `mpv_path` is a nonempty
executable name or path. Telemetry is unsupported; enabling it is rejected.
Themes use `colors.toml` or `themes/<name>.toml` in the config directory;
sending `SIGUSR1` to the running podmutt reloads the active theme live.
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

`j/k` or Up/Down select; `g/G` go first/last. **h/Left goes back; l/Right opens**
a show or inspects an episode without starting playback. Tab/Shift+Tab cycles
**Library → Episodes → Queue**; `1-5` still focuses any pane directly.
Details and Player are no longer mandatory Tab stops. In lists, Ctrl-d/Ctrl-u
moves half a visible page and PgUp/PgDn moves a full page. Esc clears an applied
filter first, then goes back; h goes directly back. `/` filters the focused list; Enter applies, Esc cancels, Ctrl+U
clears input. `?` opens help; `e` opens settings; `q` or Ctrl+C quits. Printable keys stay literal
while typing a filter.

With **Details (4)** focused, `j/k` or arrows scroll one line, Ctrl-d/Ctrl-u
scroll half a page, `g/G` or Home/End reach the top/bottom, and PgUp/PgDn scroll
a full page. These keys do not change episode selection or playback.

The wide layout reads left to right: **Library (1)**, **Episodes (2)**, **Queue (3)**.
Details sits below Episodes; the compact **Now playing** card sits above Queue and
normally uses six rows. Long descriptions scroll without moving controls. Selection
uses a highlighted row, never a `>` cursor; the active pane has an accented border.
Errors/warnings may reserve a few additional player rows; compact layouts adapt
without hiding current identity or transport.

**,/. seeks backward/forward 15 seconds from any pane** after an episode
loads, preserving pause state and resume position. h/l never seeks. Seek hints appear when space
permits; Tab always leaves Player. Native macOS builds expose matching 15-second
Control Center skip commands; absolute system scrubbing remains disabled. Linux
TUI seeking works independently of MPRIS (which still advertises no seek support).
See [seeking policy](docs/adr/021-shared-seeking.md).

Press **5** for **Now playing**, the fixed player pane. Here Left/Right chooses **Play/Pause, Stop, Next**; Enter
activates. Space/s/n still work as shortcuts. The player shows current title,
status, real-time elapsed/total/remaining and a fractional read-only progress bar;
unavailable buttons are dimmed. The presentation clock updates at 4Hz without
per-frame IPC/database work, freezes paused/stopped, and limits interpolation to
two seconds if actual samples stall. Unknown duration stays unknown. Queue-only
updates preserve the clock; real samples/seeks correct it. See
[progress policy](docs/adr/022-realtime-player-progress.md).
Playback errors remain visible independently of the selected description.
Decoration is off by default. With `player_decoration = true`, spacious wide
layouts reserve two optional rows for the labelled **Decorative** pattern and
hairline visualizer. They are not measured audio analysis. Animation freezes on
pause and clears on stop; hidden, compact, modal, or disabled decoration has no
continuing animation timer. Elapsed time appears once, with total/remaining only
when space permits.

Below 100 columns or 24 rows, the persistent player sits above the focused list.
Queue/Details replace that list when focused; Esc returns to the originating list.
Compact terminals show one library pane plus controls. Below 30×12, resize
guidance replaces the panes. See [fixed player layout](docs/adr/014-fixed-player-pane.md).

`N` adds a public RSS/Atom feed or podcast website URL. **Library** contains only
**All episodes**, a muted, non-selectable **Subscriptions** heading, and subscribed
shows. The heading adds no navigation stop; short panes omit the blank spacer.
Highlighting All episodes or a show immediately scopes Episodes; Enter/l focuses
them. From Library or Episodes, `o` cycles newest, oldest, and title ordering.
There is no extra Default option, Not started/Continue destination, or `v` toggle.

All episodes includes subscribed history (including completed episodes), saved
unfinished unsubscribed work, pending work, and current audio. Merely previewing
a show does not add it. Browsing never starts audio; explicit Enter resumes saved
work without subscribing. `u` toggles the selected show's subscription; `c` toggles
episode completion. `R` refreshes manually. Cached content opens before background
refresh; unsubscribe retains cached episodes/history and queued work. Episode rows
distinguish pending membership, completion, and current playback. PgUp/PgDn scroll
details. See [library UI policy](docs/adr/009-library-ui.md).

**Space pauses/resumes current playback everywhere**, never the highlighted episode.
With no current episode it shows guidance instead of starting the queue.
**Enter plays the selected Episodes/Queue item** (and promotes a queued row);
repeated Enter keeps it playing. **i opens Details; Esc returns to the originating
list and selection**. Library, Queue, and RSS previews share one episode card:
title/show, known date/duration, listening state, relevant progress/queue membership,
then a plain-text description. Identity stays visible while the description
scrolls; long metadata remains accessible in the reading area. Playback/refresh
does not replace an inspected episode or reset its reading position.
In Details, **Enter plays/resumes the inspected episode, a queues it, and c toggles
completion** when supported. Direct **4** pins the shown episode too. These actions
never follow a different underlying list row; Esc restores the inspected selection
when it is still visible. Preview play/queue does not subscribe; preview completion
requires subscription. Space still controls current audio. `s` stops and `n` advances to the newest pending
episode by publication time; natural completion uses the same priority, independent
of insertion/display order. Equal or unknown dates retain queue order as a tie-breaker.
**Queue projects current audio alongside pending work**, with a current-status label,
row number, title and show—not duplicate clocks or percentages. Current audio is
not duplicated in the actor's pending queue and cannot be removed by pending edits.
The header shows visible count (or matching/total while filtering).
Playing a different episode preserves interrupted playback; completion keeps history.

Queue `o` toggles newest-first and channel alphabetically + newest-first. Sorting is
presentation-only: it preserves selected identity and never changes persisted order,
playback priority, or current audio. There are no manual J/K moves. `a` enqueues one
item; `A` enqueues visible Episodes. Explicit additions append to the display tail,
become selected, and focus Queue without autoplay. A sort toggle resets manual-tail
overrides. Filters and modal input are not interrupted; Queue focus waits until they close.
**d or Delete removes pending work immediately; u undoes the last removal**, even
when no pending work remains. Undo merges that one
episode into the live queue without rewinding playback or other edits. It lasts
for this session; playing/re-adding/completing that episode or explicitly clearing/
replacing the queue invalidates it. `u` still subscribes/unsubscribes in Library.

`D` clears pending items while keeping current playback and requires explicit
`y`; Esc/n cancels. A changed queue invalidates clear confirmation. Queue edits do
not change current playback/history. Status shows position, optional duration, and
persistent playback failures. See [player/queue policy](docs/adr/011-player-queue-ui.md).

Subscribed shows consider their two latest cached releases at startup,
on subscription and after successful refresh (including 304). Arrivals append
without autoplay or older historical fallback. Completed, current and already-seen
releases are not re-added after removal/restart. Ad-hoc shows
never auto-enqueue. Unsubscribe keeps queued work/history; resubscribe keeps these
receipts. See [arrival policy](docs/adr/020-subscribed-latest-autoqueue.md).
Schema migration 2 preserves listening state; older binaries cannot open it—use a
consistent backup before upgrading, not a forced downgrade.

## Discover and subscribe

An empty profile opens **Discover**, automatically loading Apple's **US top-25**
shows. Otherwise startup stays cached/local-first. With an unfinished saved current
episode, the application explicitly resumes at its saved position and aligns the
show, Episodes and Queue selections once. Unsubscribed saved audio remains available
through All episodes without subscribing. Progress does not snap browsing back.
Without saved current audio, opening Library does not start pending work.

`/` searches the focused list; **f Find podcast** opens the shared name/feed/website
entry dialog. Advanced `U` RSS/website preview and `t` Apple US top 25 remain available.
The chart is not the whole catalog; search accepts arbitrary show terms.

1. Press `b` to switch My library / Discover (Apple initially).
2. Select an Apple show and press Enter to preview its RSS episodes. Focus stays in
   Library; nothing is subscribed, played, or downloaded as audio at this stage.
3. Press **Tab/2 to reach Episodes, then Enter to play without subscribing**, or `a`/`A` to enqueue the selected/
   visible preview episodes. Only explicit selections are cached; show subscription
   and feed refresh validators are unchanged. **i** inspects details without caching
   audio metadata; **Space** always pauses/resumes current audio, even while browsing
   a different preview.
4. Press `u` only if you want to subscribe. The complete preview is then cached,
   preserving ad-hoc playback identity/history. Browsing/preview alone stays read-only.

**U** accepts an HTTP(S) podcast feed or website URL and opens the same read-only
preview, with explicit Enter/a/A play/enqueue and u subscribe. **N** remains the
separate add-and-subscribe shortcut. Esc cancels requests; stale replies are ignored.
See [entry-point policy](docs/adr/023-discovery-entry-points.md).

`f` accepts a podcast name **or a feed/website URL**. Pasting an HTTP(S) URL opens
the same read-only preview as `U`, regardless of the selected search source; it
never subscribes or starts playback. URL-like invalid input stays in the editor
rather than being sent to a directory. `U` and `N` remain available.

For names, `f` uses source-labelled search in the current view.
Tab cycles **Library / Apple / Podcast Index**; Enter submits, Esc cancels, Ctrl+U clears. Library searches subscribed show names
and cached episode titles/descriptions, including completed history. Apple searches
shows only—not a global episode index. `/` still filters only the focused list.
Esc clears library search or leaves a preview. An empty Apple query restores the
chart; `R` retries/reloads directory results. Quota waits can take a few seconds.

Apple access is public/no-key, not an import of Apple account subscriptions.
Network failure leaves your library usable with `b`; errors never erase it.
### Optional Podcast Index provider

Select Podcast Index explicitly with **f → Tab** in the source-labelled search
dialog. Supply your own `PODMUTT_PODCAST_INDEX_KEY` and
`PODMUTT_PODCAST_INDEX_SECRET` via a local secret manager/environment; no shared
key is included. Missing access never silently falls back. Apple/RSS still need
no key. Read [credential setup, current terms and limits](docs/podcast-index.md)
before use; fixture CI does not imply a successful live authorized search.

### Artwork

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
