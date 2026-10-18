# ADR 017: Checkpoint/reap before process cancellation

Status: accepted; regression found during combined #13/#14/#16/#17 PTY validation

The playback actor used its own cancelable context for mpv's exec.CommandContext.
Engine.Close (or parent cancellation) therefore killed mpv concurrently with the
actor's final Position/checkpoint and IPC-close/reap. Quitting after real playback
could lose the final position and return `signal: killed` instead of success.
Direct Player.Close tests and fake players ignoring contexts did not expose it.

Give each lazily created player a private cancelable lifetime detached from actor
cancellation, retaining context values. Actor cancellation still initiates bounded
cleanup: query/checkpoint with a fresh deadline, close IPC and reap (Player.Close
retains its kill fallback), then cancel/release the private process lifetime.
Failed/nil factory and player-failure paths release it too. No process is detached:
parent death closes the inherited socketpair, and explicit cleanup remains owned
by the same actor. There is still one foreground profile/actor/player.

Regression tests use a context-sensitive player to check both Engine.Close and
parent cancellation, a fresh final position, reap-before-cancel and failed-factory
release. The required real-mpv/null-audio job now additionally crosses full
session -> actor -> process cleanup. Combined PTY fake-IPC smoke verifies ordinary
quit exits zero and leaves no player/control endpoint.
