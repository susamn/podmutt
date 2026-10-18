# ADR 016: Public macOS MediaPlayer bridge

Status: prototype; native compilation/runtime and physical-key verification pending

Use the public MediaPlayer.framework MPRemoteCommandCenter and
MPNowPlayingInfoCenter, plus AppKit/Foundation. Do not use private MediaRemote
symbols or global event taps/Accessibility/Input Monitoring permissions.
Metadata is limited to current title, elapsed/duration and playback state, as
needed for the system to advertise command ownership. There is no extra app UI,
Dock icon, separate helper, daemon or second player.

Darwin CGO builds pin the process's original thread during init. Only the actual
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
Teardown cancels/drains the worker, unregisters handlers, clears system metadata,
and removes its integer-token callback registry entry before actor/storage close.
Late native callbacks find no Go registry entry; C never stores Go pointers.
Copied handlers retain their Objective-C owner; its active flag is atomic.

Native builds need Apple's SDK/clang, CGO, a bundle identifier and linked public
frameworks. `make native-macos` creates a terminal-launched .app executable with
an ad-hoc development signature. This is NOT a notarized/release signature.
No special entitlement is requested. Release signing/notarization is #18 and
must not be inferred from a locally successful ad-hoc signature.

Existing pure-Go Linux/Darwin cross-builds remain possible. Darwin CGO-disabled
binaries explicitly warn that native media controls are unavailable; they are
not equivalent to the bundled native build and must not be advertised as full
macOS release artifacts. Native runner recipes and manual verification are in
`docs/macos-media-check.md`. Linux tests prove mapping/lifecycle only, not Objective-C
compilation, framework behavior, terminal focus, permission behavior or key delivery.
