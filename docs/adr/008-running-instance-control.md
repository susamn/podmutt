# ADR 008: Private foreground controls

Status: proposed in #7.

Expose play/pause/stop/next/status through control.sock (0600) inside the
profile's validated private runtime directory. Only the foreground owner binds
it, after acquiring the profile lock. Refuse live/unsafe endpoints; remove only
verified stale sockets. Cleanup removes only the inode this server created.

Use one newline-delimited JSON request/response per connection, strict fields,
4 KiB requests, 1 MiB responses, 16 simultaneous clients, and 5-second deadlines.
Cancel accepted connections/actions on session shutdown. Status includes IDs,
titles, position/duration/completion and queue order, not stream URLs/descriptions.

Keyboard Space (toggle), s (stop), n (next), and CLI controls use the same action
dispatcher and playback actor. Selected-episode playback remains in #11. The
foreground session owns one SQLite handle/actor; restore never autoplays.

CLI clients resolve/validate paths only: no configuration load, directory
creation, ownership acquisition, database writes, or player launch. Missing or
refused sockets mean "not running". Exit codes: 0 success, 1 action/setup/protocol
failure, 2 usage error, 3 not running. Only status prints JSON on success.
