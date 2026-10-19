# ADR 040: Explicit library destinations

Status: accepted for #39 (library slice).

My library lists New episodes, Continue listening, All subscriptions, then
subscribed shows. Highlighting a destination scopes Episodes immediately;
Enter/l focuses the resulting list. A show always includes its completed history.
Remove the independent `v` Show/New mode. Search results retain their labelled
scope and omit navigation destinations.

Continue means saved position > 0 and not completed, including unsubscribed and
explicitly played previews. Reuse stored history/episode identities; do not
subscribe, queue, or autoplay by browsing. Without subscriptions, existing
unfinished history starts in Continue rather than Discover, still stopped.

Use one canonical sidebar destination and version/cancel cached reads. Restore
that destination after discovery/search, fall back safely after unsubscribe, and
keep pending add selection until its rows arrive. Query existing listening state;
no schema migration or new dependency. Queue policy remains separate.

This extends ADR 009 and replaces its Show/New toggle. New retains the existing
uncompleted-subscribed definition; it does not introduce release-date cutoffs.
