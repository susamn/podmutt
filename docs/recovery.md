# Backup, recovery and migration safety

Podcast metadata/history is local data. There is no cloud sync or automatic
recovery from a deleted database. Back up before upgrading or experimenting with
migrations; keep the previous binary and a separate known-good snapshot.

## Locate and stop

Quit the foreground TUI (not `podmutt stop`, which only stops audio). `podmutt
status` should return exit 3. Do not start another owner or SQLite editor during
backup/restore. Never delete `tui.lock` to bypass ownership.

Database paths:
- Linux: `${XDG_STATE_HOME:-$HOME/.local/state}/podmutt/podcasts.db`.
- macOS: `$HOME/Library/Application Support/podmutt/podcasts.db`.

A live/WAL-mode database is **not** safely backed up by copying just `podcasts.db`.
Committed data may still be in `-wal`; independently copying DB/WAL/SHM can also
mix transactions. Use SQLite's backup API/`.backup`, or `VACUUM INTO`, for a
consistent snapshot. The tests exercise a snapshot including committed WAL data.

## Create a snapshot

With the optional `sqlite3` CLI installed, choose a new backup path (examples
assume no quote characters in the path):

```sh
umask 077
DB="$HOME/.local/state/podmutt/podcasts.db" # adjust for XDG/macOS
BACKUP="$HOME/podmutt-backup-$(date +%Y%m%d-%H%M%S).db"
test -f "$DB" && test ! -e "$BACKUP" || exit 1
sqlite3 "$DB" ".backup '$BACKUP'"
chmod 600 "$BACKUP"
sqlite3 "$BACKUP" 'PRAGMA integrity_check; PRAGMA foreign_key_check; PRAGMA user_version;'
```

Expect `ok`, no foreign-key rows and the supported schema version (currently 1).
Test a backup against a **separate profile** before relying on it. A backup from
a future schema is not a downgrade: restore its matching binary instead.

## Restore without destroying evidence

1. Quit all owners/editors. Preserve the existing DB **and any DB-WAL/SHM files**
   together in a private recovery directory before changing anything. Do not
   delete a WAL in an attempt to repair a failed startup.
2. Validate the backup's integrity/schema as above. Never overwrite a suspected
   good backup with damaged/newer data.
3. After preserving the old set, move it out of the state directory. Install a
   **copy** of the validated standalone snapshot as `podcasts.db`, owned by the
   current user, mode 0600, in the existing private 0700 profile directory.
   Do not bring the old WAL/SHM alongside the replacement snapshot.
4. Start podmutt. Current episode/checkpoint/queue restore **stopped**; no playback
   starts until an explicit action. Check subscription/history/queue before play.
5. Keep the preserved old set and backup until the new profile is verified.

## Crash/migration guarantees and limits

SIGKILL tests leave committed WAL state without Store.Close, then reopen and
verify integrity, subscriptions, completion, position/current and queue order.
Snapshots are checked independently after the source changes. Failed upgrades
roll back data, DDL and user_version atomically; unknown/newer versions fail
without attempting a downgrade. Existing tests also reject unsafe file modes,
symlinks and duplicate foreground ownership.

Orderly quit keeps mpv alive through the final position/checkpoint and IPC
close/reap, then releases its process context (ADR 017). The real-mpv CI gate
checks this full session boundary, not just direct Player.Close.

Only committed checkpoints survive a crash. Playback normally checkpoints every
five seconds and on pause/stop/replacement/EOF/quit, so the last few seconds may
be lost. Abrupt termination is not completion. Hardware/filesystem failure,
manual file edits and restoring incompatible binaries are outside that guarantee.
Do not mistake passing Linux recovery tests for native macOS verification.
