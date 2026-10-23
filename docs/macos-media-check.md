# Native macOS media verification — pending

Run on macOS 13+ with Go matching go.mod, Xcode command-line tools, and mpv.
No Linux cross-build can satisfy this check. Implementation runs on Linux;
a user-provided arm64 Mac is performing native acceptance checks.

```sh
make native-verify # preflight + native block/real-mpv tests + check/race + app
bin/podmutt.app/Contents/MacOS/podmutt --version
bin/podmutt.app/Contents/MacOS/podmutt
```

Launch the bundled executable **from Terminal**, not Finder. An ad-hoc signature
is local development only. Existing `make build/dist` use CGO=0 and deliberately
omit the native bridge; the TUI displays an unavailable warning on those builds.
`native-media-test` builds a bundled `-tags=nativecheck` test executable with a
separate bundle identity. It invokes the exact registered Objective-C blocks
through CGO for Play/Pause/Toggle/Stop/Next and ±15-second skip commands, inspects public metadata/control flags,
checks repeated registration and a retained handler delivered after teardown.
The test-only C hooks are excluded from normal builds. `native-verify` runs the
bundled test and real-mpv tests first and combines their coverage profiles with
ordinary Go tests before enforcing the unchanged 95% statement gate. `make check` on its own does not run
the native bridge tests and may report lower Darwin coverage. This is not a Go callback
shortcut, but it still does **not** press physical keys or prove system dispatch
or another player's arbitration. Full native-handler execution remains unverified
until this command succeeds on a real Mac.

`native-preflight` records OS/architecture, Go and Apple SDK/compiler availability.
`native-verify` requires mpv and also runs the real null-audio session/actor/process
checkpoint-before-reap tests merged in !19. Go 1.27's macOS minimum is 13; the bundle
and native compiler/linker settings are aligned to that version.

Record OS/architecture, Go/Xcode/mpv versions, terminal and commit SHA, then check:

- [ ] Native CGO build and common check/race tests pass on amd64 and arm64 hosts
      (one host architecture alone is not evidence for the other).
- [ ] Native framework test exits cleanly without a window/Dock icon.
- [ ] Help/version/CLI do not register handlers or start a GUI event loop.
- [ ] Subscribe/play from the TUI; physical Play/Pause and Next reach the actor.
- [ ] Paused/playing/stopped state and current title/time are correct in system UI.
- [ ] Loaded playing/paused episodes expose ±15-second Control Center skip controls;
      both change actual position and saved resume without changing pause state.
- [ ] Disabled Next (no queue), unready skip, unsupported previous/absolute system
      scrubbing/continuous seeking do not alter history.
- [ ] Controls still work when another app/terminal has keyboard focus.
- [ ] No Accessibility/Input Monitoring request occurs; record any other prompt.
- [ ] Quit/signal/failed startup remove handlers and Now Playing metadata; no
      detached audio remains. Relaunch and repeat.
- [ ] With another player active, test ownership/arbitration and return after quit.
- [ ] Repeat with a CGO=0 binary: nonfatal unavailable warning and ordinary
      keyboard/CLI playback remain functional.

If standalone CLI/bundle activation or public-framework command delivery does
not work on the tested OS, keep #16 open and revise the prototype. Do not silently
substitute a private API/global key hook or mark this complete from a cross-build.

An optional GitLab shell runner tagged `macos` can run the native job when
`PODMUTT_NATIVE_MACOS=1` is configured. Absent that runner, the job is not scheduled;
this is an explicit verification gap, not a green native result. All three current
project runners were checked and are Linux amd64, without the `macos` tag. Configure
a native shell runner with that tag and set `PODMUTT_NATIVE_MACOS=1` only once it is
available. The job records its output as `coverage/native-macos.log` and retains the
development app artifact; neither is evidence until the native job actually runs.

## First user-provided arm64 run

Reported host: macOS 27.0.1 (26A434), Go 1.27.2 darwin/arm64, Xcode SDK/compiler
available. Ordinary native CGO packages compiled/linked and vet passed, but the
common app tests stopped before dedicated native-handler/race/real-mpv checks:

- macOS's long `/var/folders/.../T` TMPDIR plus the standard profile suffix exceeded
  the 104-byte Unix socket path limit. Isolated test homes now use atomically
  created private `/tmp/pma-*` directories; a long-TMPDIR regression covers this.
- Bridge-local deployment flags did not cover runtime/cgo and other CGO objects,
  producing host-SDK-27 versus linker-13 warnings. Darwin Make recipes now export
  deployment target 13.0 and explicit C/C++/link flags for all CGO packages; changed
  flags invalidate cached objects without deleting the user's entire build cache.

Rerun `make native-verify` after pulling the fix. These observations are partial
compile/link evidence, not a successful native acceptance run or physical-key test.
