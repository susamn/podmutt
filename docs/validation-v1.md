# V1 validation ledger

Status: automated Linux coverage implemented; native/physical checks pending.
This ledger is not a release approval and #17 must remain open until its native
and feature dependencies are verified. Features #13/#14/#16 are separate MRs.

## Repeatable gates

- `make check`: format/modules/vet, all tests, >=95% aggregate first-party Go
  statement coverage (including CLI code).
- `make race`: native Go race detector, all tests.
- `make recovery-test`: local HTTP feed/website preview + explicit subscribe,
  conditional refresh, description search, queue/selected playback, private-socket
  pause/resume, EOF completion/advance, quit/checkpoint, stopped offline restart,
  cached completed search and explicit resume; no external feeds/audio.
  Also SIGKILL/WAL reopen, consistent snapshot/restore and failed migration rollback.
- `make mpv-test`: real mpv/null-audio lifecycle, load/pause/resume/stop/EOF/error,
  bounded shutdown and abrupt owner-death/IPC loss. It also crosses real
  session/actor/process quit to catch premature context kills and final checkpoint
  loss (ADR 017). Required Linux CI job.
- `make mpris-test`: private dbus-daemon protocol/actions/metadata/shutdown.
  Required Linux CI job. This does not press desktop media keys.
- `make dist`: all-package Linux/Darwin amd64/arm64 cross-compilation. Cross-builds
  do not prove native runtime, audio devices or platform media controls.

Existing focused suites cover discovery quotas/HTTP bounds, refresh cancellation,
redacted URL failures, private endpoints/permissions/lock safety, stale queue edits,
input guards and cleanup. New recovery fixtures use only loopback HTTP, temporary
private profiles, bounded child processes and a fake player. External podcast
availability is not a test prerequisite.

## Manual/native matrix — do not infer these from CI

| Check | Linux | macOS amd64 | macOS arm64 |
|---|---|---|---|
| Native TUI startup/quit, offline cached restart | PTY smoke performed | pending | pending |
| Native real-mpv/null audio lifecycle/abrupt death | CI required | pending | pending |
| Physical media keys + other-player arbitration | pending: docs/linux-media-check.md | pending: docs/macos-media-check.md (#16) | pending: same |
| Real Kitty display/resize/overlay/quit cleanup | pending: docs/artwork-check.md (#13) | pending if Kitty used | pending if Kitty used |
| Native bridge build/event-loop/registration | not applicable | pending (#16) | pending (#16) |
| Consistent backup/restore + failed migration | automated | pending native run | pending native run |

Record OS/architecture, terminal/mpv/toolchain versions, commit SHA, commands and
outcomes when completing a row. A simulated Kitty reply is protocol/PTY coverage,
not a real Kitty result. A callback invoked in a fake driver is mapping coverage,
not macOS framework/key coverage. Omitted opt-in native jobs are not passes.

## Release prerequisites

Before #18: merge/verify all feature MRs, run this ledger against the resulting
main commit, resolve native bridge build/packaging/signing implications, record
missing physical/native checks, and test a clean install/upgrade/uninstall.
Recovery instructions are in `docs/recovery.md`. Do not mark the release or #17
complete solely because independent branch pipelines are green.
