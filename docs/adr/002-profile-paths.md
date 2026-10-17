# ADR 002: Private profile paths and ownership

Status: accepted.

Linux uses `$XDG_CONFIG_HOME/podmutt/config.toml` and
`$XDG_STATE_HOME/podmutt`, defaulting to `~/.config` and `~/.local/state`.
Runtime files use `$XDG_RUNTIME_DIR/podmutt`, or the private state directory's
`run` subdirectory when unavailable. Relative XDG values are ignored.
macOS uses `~/Library/Application Support/podmutt`, with `config.toml` and a
`run` subdirectory. These paths represent the default user profile.

State and runtime directories must be private (0700); reject public directories
and final-component symlinks; require current-user ownership.
Create lock files with 0600 and reject public files
and symlinks. Kernel advisory flock admits one cooperating TUI and releases on
process exit, including crashes. Keep the lock file permanently: deleting it
would allow another process to lock a different inode. Stale file contents are
irrelevant; never use PID probing to decide ownership.

The future foreground TUI resolves paths, loads configuration, prepares private
directories, acquires the lock, and holds it through cleanup. A second owner gets
`ErrAlreadyRunning`. CLI clients only resolve paths and contact the application
endpoint; they never prepare state or acquire the lock. The current bootstrap
has no TUI lifetime and intentionally does not acquire ownership.

Use BurntSushi/toml for full TOML syntax and type checking; the standard library
has no TOML decoder. Reject unknown keys and invalid values instead of silently
ignoring configuration mistakes. No additional locking dependency is needed on
Linux/macOS. Native macOS filesystem/locking checks still require a macOS runner.
