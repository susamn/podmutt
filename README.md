# podmutt

A Linux/macOS podcast TUI inspired by lazygit and mpdtui. Planned stack: Go,
Bubble Tea, SQLite, and mpv. Licensed under [MIT](LICENSE).

**Status:** development bootstrap only. The executable supports `--help` and
`--version`; podcast playback and the TUI are not implemented yet.

## Development

Requires Go 1.27.2+, Make, and a C compiler for race tests. Runtime playback will
require a separately installed mpv; no Go toolchain is needed for release users.

```sh
make check       # formatting, module consistency, vet, tests, >=90% coverage
make race        # native race-detector tests
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

## Configuration and profile

The profile package is ready for the future TUI; the bootstrap does not yet load
configuration or own a running profile. An absent `config.toml` uses these defaults:

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

The storage package is ready for the future TUI: `podcasts.db` holds subscriptions,
episodes, history, current episode, and queue. The bootstrap does not open it yet.
Do not copy only an open database file for backup; SQLite WAL sidecars may contain
recent writes. Backup tooling is tracked in #17.
