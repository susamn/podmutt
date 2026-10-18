# ADR 020: Latest arrivals without autoplay or resurrection

Status: implemented for post-v1 #22.

Reconcile cached subscriptions at foreground startup, after successful explicit
subscription changes, and after successful feed refresh results (including 304).
The playback actor serializes arrivals with manual queue edits. It appends, never
promotes, starts mpv, changes current playback or sorts existing pending work.
Ad-hoc/unsubscribed feeds never produce automatic arrivals; unsubscribe retains
existing queue/history and receipts, and resubscribe does not reset receipts.

For each subscribed feed choose only its newest cached episode: publication time
descending, then stable feed identity ascending for missing/equal dates. Never
fall back to older episodes when that head is completed/already processed. With
no usable publication dates, identity provides determinism, not guessed chronology.

Schema migration 2 adds an indexed per-feed newest lookup and `autoqueue_seen`
receipts keyed by episode ID. Receipt plus queue append commit in one transaction.
Current, completed and already queued heads consume receipts too, so refresh,
restart or later completion changes cannot undo an explicit removal. A new head
can append once; unchanged cached feeds/304 do not flood the stack with backlog.
Migration preserves existing queue/current/history; older binaries cannot open
the newer schema. Back up consistently before upgrading; do not force downgrade.

Tests cover startup without autoplay/audio requests, subscription wiring, 304,
manual/automatic serialization, stable ties, non-subscribed exclusion, current and
completed heads, removal/resubscription/reopen, atomic rollback/retry, v1 migration
and failed future upgrade rollback. No new dependency or background daemon.
