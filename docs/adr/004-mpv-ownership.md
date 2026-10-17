# ADR 004: Own mpv through inherited IPC

Status: proposed in #5.

Use a Unix socketpair, passing its child endpoint as fd 3 with
`--input-ipc-client=fd://3`. mpv documents that it exits when this connection
closes. The kernel closes the parent's endpoint even on SIGKILL, avoiding
Linux-only parent-death signals, a filesystem socket, and a watchdog process.
Close original/duplicate endpoints promptly; mark descriptors close-on-exec
under Go's fork lock. Disable user mpv configuration and script loading.

Commands have request IDs, cancellation, deadlines, and a 1 MiB message limit.
Observe position/pause/duration; preserve end-file reasons exactly. Never drop
completion events silently: disconnect on event-buffer overflow. On shutdown
close IPC, wait, then kill/reap an unresponsive child within bounded time.

Use named loadfile arguments (`name`, `url`, `flags`, `options`) to avoid the
positional index added in mpv 0.38. Per-file start/pause options bind resume and
play to that load rather than racing a later seek. No extra dependency.

`make mpv-test` checks real mpv with generated WAV fixtures and null audio,
including abrupt owner death. Linux CI installs mpv explicitly for these checks.
Native macOS runtime validation remains required in #17; cross-compilation is
not proof of lifecycle behavior.

Reference: https://mpv.io/manual/stable/#options-input-ipc-client
