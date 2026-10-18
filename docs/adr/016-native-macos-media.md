# ADR 016: Public macOS MediaPlayer bridge

Status: prototype; native compilation/runtime and physical-key verification pending

Use the public MediaPlayer.framework MPRemoteCommandCenter and
MPNowPlayingInfoCenter, plus AppKit/Foundation. Do not use private MediaRemote
symbols or global event taps/Accessibility/Input Monitoring permissions.
Metadata is limited to current title, elapsed/duration and playback state, as
needed for the system to advertise command ownership. There is no extra app UI,
Dock icon, separate helper, daemon or second player.

Darwin CGO builds pin the process's original thread during init. Native startup
first checks the original main thread and bundle identity; unavailable/unbundled
startup falls back to the ordinary TUI without initializing AppKit. Nested event
loops are rejected. Only the actual
foreground start callback enters a non-activating NSApplication event loop;
Bubble Tea/profile ownership run in a Go goroutine. CLI/help/version do not start
AppKit or register media targets. Framework registration/metadata/removal are
marshaled onto the main queue. Foreground completion wakes/stops the native loop.

On Darwin, private mpv starts with `--input-media-keys=no` so it does not register
a competing command center. Native checks must confirm ownership/arbitration.

A bounded eight-action queue dispatches Play/Pause/Toggle/Stop/Next to the shared
actor, with three-second action deadlines. Unknown/saturated/closed dispatch
fails without blocking the native callback. An independently cancelled Watch
subscription publishes metadata/capabilities; source closure disables acceptance.
Metadata publication failures and actor-watcher closure automatically cancel/drain
the worker and remove targets, even before an explicit Close. Partial registration
and framework exceptions clean up nonfatally without logging metadata.
Teardown unregisters handlers, clears system metadata,
and removes its integer-token callback registry entry before actor/storage close.
Late native callbacks find no Go registry entry; C never stores Go pointers.
Copied handlers retain their Objective-C owner; its active flag is atomic.

Go 1.27 requires **macOS 13+**, per the [official platform minimums](https://go.dev/wiki/MinimumRequirements).
The bundle and native compiler/linker minimum agree on 13.0; macOS 11/12 are not
supported by this toolchain.

Native builds need Apple's SDK/clang, CGO, a bundle identifier and linked public
frameworks. `make native-macos` creates a terminal-launched .app executable with
an ad-hoc development signature. This is NOT a notarized/release signature.
No special entitlement is requested. Release signing/notarization is #18 and
must not be inferred from a locally successful ad-hoc signature.

Existing pure-Go Linux/Darwin cross-builds remain possible. Darwin CGO-disabled
binaries explicitly warn that native media controls are unavailable; they are
not equivalent to the bundled native build and must not be advertised as full
macOS release artifacts. Native runner recipes and manual verification are in
`docs/macos-media-check.md`. The `nativecheck` build tag includes diagnostics that
invoke the actual registered Objective-C blocks through CGO, inspect public system
metadata/capabilities and exercise a retained late handler after teardown. They
are absent from normal builds and do not synthesize physical/system media keys.
Linux tests prove mapping/lifecycle only, not Objective-C
compilation, framework behavior, terminal focus, permission behavior or key delivery.
