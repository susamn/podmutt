# ADR 006: Serialized playback service

Status: proposed in #6.

Use one foreground event loop for player events, user/CLI actions, and 5-second
checkpoints. Restore current episode/queue without autoplay; start mpv lazily.
Expose copied state snapshots and coalesced updates, not mutable player handles.
The service is ready for #7/#11; the shell does not instantiate it yet.

Persist on pause, stop, replacement, EOF, and close. Completion is explicit:
natural EOF or manual marking. Marking does not change transport/queue order.
Replay of a completed episode starts at zero and clears its completion mark.
Next skips without marking; empty queue stops. Direct play removes only the
chosen episode from the queue, preserving all other entries in order.

Read mpv's playlist entry ID after each replacement load. Match end-file events
to that ID; accept property updates only after the corresponding start-file.
This prevents buffered events from an old episode changing the new one.

Player/background persistence failures stop playback, retain the remaining
queue, and leave a persistent error in state; no automatic retry or skip.
Invalid queue/manual-history edits return errors without interrupting playback.
Playback and database operations cannot be one atomic transaction: if a write
fails after a load, stop mpv instead of allowing untracked playback to continue.

Cancel actions with the session. Bound action/cleanup operations, checkpoint
known progress, close/reap the player, then close updates. UI and CLI must use
this service for transport and queue actions; neither owns another player.
