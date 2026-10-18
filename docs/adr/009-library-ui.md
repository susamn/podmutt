# ADR 009: Cached library and explicit edits

Status: accepted in #10; discovery/list visibility extended by ADR 012.

Share the foreground session's SQLite/playback services. Read cached rows before
starting network refresh. Query/mutate asynchronously; version cached-read
results and cancel superseded reads so rapid navigation cannot apply stale data.
Refresh results invalidate incrementally, preserving selection by episode ID.

Originally listed all cached shows; ADR 012 separates My subscriptions from
Apple Discover. Unsubscribed metadata/history remains stored, not listed in My
subscriptions. [+] marks subscribed shows; read-only previews live in Discover.
Enter opens a show's episodes. v switches Show/New (uncompleted subscribed
episodes); o cycles newest/oldest/title. u toggles subscription, c toggles the
selected episode's completion through the playback actor, independent of queue
and transport. N accepts an RSS/Atom URL; R requests a background refresh.

Validate/fetch new URLs before persisting; reuse the refresher's HTTP transport
and limits. Cache episodes before validators and subscribe last. Resubscribing
an existing URL preserves metadata/history. Retain cache on network failures;
keep per-feed errors visible until that feed succeeds.

Use best-effort stdlib HTML-to-text conversion and the existing terminal escape
sanitizer; malformed markup is literal text, never interpreted. Wrap details and
scroll with PgUp/PgDn. No widget/HTML/parser dependency is added. Selected-episode
playback/queue bindings are in ADR 011; website URL discovery is in ADR 012.
