# ADR 007: Bounded foreground feed refresh

Status: proposed in #9.

Use encoding/xml for UTF-8/ASCII RSS 2, RSS 1/RDF, and Atom. Normalize public
audio enclosures, feed-scoped GUIDs, relative/xml:base URLs, common artwork,
dates, and durations. Skip non-audio/unusable entries and duplicate identities.
Limit decoded responses to 8 MiB and feeds to 10,000 entries. Unsupported
charsets/formats fail visibly without deleting cached data; do not hand-write
a general encoding converter or add a feed library before evidence requires it.

Use conditional ETag/Last-Modified requests, a 15-second total deadline, at most
five redirects, and four concurrent requests. Reject credentials/non-HTTP URLs
at every redirect; do not disclose request URLs or body data in transport errors.
Keep the subscribed URL stable across redirects; resolve metadata against the
final response URL.

Write episodes before feed metadata/validators. Failure can cause a safe repeat
fetch, never a 304 that hides episodes not yet committed. Refresh never deletes
omitted episodes or overwrites history/subscription choices. A 200 clears absent
validators; a 304 retains absent validators and may update supplied ones.

The foreground scheduler runs immediately, every configured interval (default
30 minutes), and on coalesced manual requests. Cycles never overlap; per-feed
locks also serialize direct refreshes. Deliver incremental per-feed results;
one failure does not prevent other feeds from refreshing. Session cancellation
stops requests, workers, timers, and blocked result delivery.

This service is ready for #10 to connect to the shell. No daemon or extra
third-party parser/scheduler dependency is introduced.
