# ADR 012: Apple Discover and subscribed-library search

Status: accepted for v1

## User flow

`b` switches My subscriptions / Discover. An empty profile starts in Discover
and loads Apple's non-personalized US top-25 chart asynchronously; a profile with
subscriptions opens cached library rows first and does not contact Apple until
Discover/search is requested. Show the country/source explicitly. This is not an
Apple account import or personalized recommendation engine.

Enter on an Apple show fetches a read-only RSS/Atom episode preview, never audio.
`u` explicitly commits the preview as a subscription and opens the stored show;
no refetch is needed. Preview rows have non-persistent identities and cannot be
played/enqueued/marked complete. Existing subscriptions/history are preserved.
Unsubscribed cached rows remain stored but are not listed in My subscriptions.

`f` opens search in the current scope; Tab switches Library/Apple before explicit
Enter submission. Apple searches shows only. Library searches subscribed show
names plus cached episode titles/descriptions, including completed history; no
remote episode index. `/` filters the current list locally. Esc clears library
search or exits a preview; an empty Apple query returns to the top chart. `R`
retries the current directory request. Switching scope/view cancels outstanding
work; generation/selected-identity checks reject late replies. All commands use
the foreground lifetime, and quit cancels network work.

## Provider/access review

Use Apple's public no-key APIs, retaining names, feed URLs and attribution links
only. Ignore promotional artwork, badges and preview-audio fields entirely.
- [Search/access and rate guidance](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/Searching.html)
- [Lookup](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/LookupExamples.html)
- [RSS chart builder](https://rss.marketingtools.apple.com/)
- [Usage terms](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/index.html)

Initial chart: `/api/v2/us/podcasts/top/25/podcasts.json` on
`rss.marketingtools.apple.com`; one batch iTunes lookup resolves chart IDs to RSS
URLs, preserving chart order. Search uses `media=podcast&entity=podcast`, max 25.
Retain the Apple Podcasts source link in previews. These technical docs do not
grant a blanket license for promotional assets or RSS media; keep the provider
replaceable. Live chart/lookup access was checked, but tests use local fixtures.

## Bounds and privacy

The archived API guidance states approximately 20 calls/minute, subject to
change, and recommends caching. Serialize all chart/lookup/search attempts at
least three seconds apart. Use a ten-minute/128-query metadata cache, a 15-second
total deadline including quota wait, max five redirects and 1 MiB decoded JSON.
Honor bounded Retry-After; no automatic retry loop. Shared IPs can still throttle.
Canceled waits do not reserve slots. Normalize/cap queries to 128 characters,
result names to 512 characters, URLs to 4096 bytes, and directory results to 25.
No keystroke queries, Apple login, telemetry, asset downloads, or new dependency.

Library search is capped at 100 episodes and retains library sorting; existing
v1 reads still materialize the subscribed cached set. An indexed SQL search can
replace that if scale demands without changing the UI/provider boundary.

## Feed/website resolution

`N` accepts feeds and ordinary websites. Reuse refresh's HTTP validation,
redirect/body/concurrency bounds and one 15-second deadline. Parse direct feeds
first; otherwise scan at most 1 MiB of HTML head for up to three explicit
RSS/Atom/RDF alternate links, resolving against the final response URL and first
base element. Ignore comments/scripts, credentials and non-HTTP links. Try each
candidate as a strict feed; do not recursively crawl. This is best-effort stdlib
HTML handling, not a browser. Unsupported/malformed sites visibly fail without
changing subscriptions/cache. Release website slots before candidate requests
to avoid nested-slot deadlocks. Store the resolved feed URL, never the website.
