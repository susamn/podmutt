# podmutt

A Linux/macOS podcast TUI inspired by lazygit and mpdtui. Planned stack: Go,
Bubble Tea, SQLite, and mpv. Licensed under [MIT](LICENSE).

**Status:** responsive TUI shell available. Run `podmutt` for empty panes and
keyboard navigation and restored playback controls; podcast browsing is not connected yet.
`--help` and `--version` work without creating profile files.

## Development

Requires Go 1.27.2+, Make, and a C compiler for race tests. Runtime playback will
require a separately installed mpv; no Go toolchain is needed for release users.

```sh
make check       # formatting, module consistency, vet, tests, >=90% coverage
make race        # native race-detector tests
make mpv-test    # real-mpv lifecycle/playback tests, null audio; requires mpv
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
  media-key testing, signing, and package publication will be added separately.

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

## Configuration and profile

The TUI loads configuration and holds exclusive profile ownership until exit.
An absent `config.toml` uses these defaults:

```toml
refresh_minutes = 30
mpv_path = "mpv"
telemetry = false
```

Refresh intervals must be positive whole minutes. `mpv_path` is a nonempty
executable name or path. Telemetry is unsupported; enabling it is rejected.
Malformed TOML and unknown keys are errors. Configuration files are never
rewritten automatically.

Linux configuration lives at `~/.config/podmutt/config.toml`, state at
`~/.local/state/podmutt`; absolute XDG overrides are honored. Runtime files use
`$XDG_RUNTIME_DIR/podmutt` or a private subdirectory of state. macOS uses
`~/Library/Application Support/podmutt/config.toml` with state beside it and a
private `run` directory. See [profile paths and ownership](docs/adr/002-profile-paths.md).

## Shell controls

`j/k` or arrows select; `g/G` go first/last. `h/l`, Tab/Shift+Tab, or `1-4`
switch panes. `/` filters the focused list; Enter applies, Esc cancels, Ctrl+U
clears input. `?` opens help; `q` or Ctrl+C quits. Printable keys stay literal
while typing a filter.

The four-pane layout switches to stacked podcasts/episodes/player below 100
columns or 24 rows. Queue/details open as overlays; Esc returns to the last
visible list. Below 30×12, resize guidance replaces the panes.

Space toggles the current episode; `s` stops and `n` skips to the explicit queue's
next episode. Selected-episode playback is tracked in #11.

## Running-instance commands

`podmutt play`, `pause`, `stop`, `next`, and `status` contact the running TUI.
Only `status` prints JSON. They never start a player or create profile files when
the TUI is absent. Exit codes: 0 success, 1 failure, 2 usage, 3 not running.
See [private controls](docs/adr/008-running-instance-control.md).

## Storage

The foreground session opens `podcasts.db` for subscriptions, episodes, history,
current episode, and queue. It restores playback without autoplay.
Do not copy only an open database file for backup; SQLite WAL sidecars may contain
recent writes. Backup tooling is tracked in #17.

## Player adapter

The playback service starts mpv lazily for the current/queued episodes. It uses
a private inherited socketpair, separate from the application control socket. Missing
mpv produces installation guidance. User mpv configuration/scripts are disabled.
Run `make mpv-test MPV=/path/to/mpv` to verify resume, pause/play/stop, EOF versus
errors, graceful shutdown, and exit after abrupt owner death. No audio device or
network podcast is needed. Linux CI runs these checks; native macOS validation
still needs a macOS host.
