# ADR 003: Local SQLite storage

Status: proposed in #4; depends on #3 / MR !2.

Use database/sql with modernc.org/sqlite (pure Go). The standard library has no
SQLite driver; this keeps current Linux/macOS cross-builds CGO-free. Accept the
driver's transitive dependencies instead of maintaining a decoder or database.

Store podcasts.db in the private profile state directory. Require a private,
current-user-owned regular database file. Use WAL, foreign keys, a 5-second busy
timeout, and one connection. The application profile lock owns write access;
CLI clients never open the store. Do not log SQL arguments or feed URLs.

Embed ordered migrations; execute pending migrations and PRAGMA user_version
updates in one transaction. Reject newer schemas rather than downgrade. Never
delete older episodes just because a refresh omits them.

Feed URLs are unique as supplied; the feed resolver handles canonical redirects.
Episode identity is feed-scoped GUID, falling back to the enclosure URL when
GUID is absent. Prefix identity types to avoid collisions. Enclosure URL changes
without a GUID cannot be reliably correlated. Metadata refresh preserves history
and local episode IDs.

Queue entries are unique episode IDs in explicit order. Replace the queue
transactionally; invalid entries roll back the entire replacement. Playback
checkpoints persist current episode and position atomically. Completion is an
independent user/player decision, never inferred by storage.

Live backup/recovery tooling is tracked in #17; do not copy only the main file
while a WAL database is open.
