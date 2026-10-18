# ADR 012: Isolate no-key show discovery from cached episode search

Status: proposed (backend implemented; UI/website resolver in progress)

## Provider/access review

Use Apple's public iTunes Search API with `media=podcast&entity=podcast`, no key,
25 results. This is a show directory, not a remote episode index.

Official archived documentation:
- [Searching](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/Searching.html)
  states approximately 20 calls/minute, subject to change, and recommends query
  caching. We serialize outbound attempts at least three seconds apart and cache
  results for ten minutes (128 queries maximum) per foreground instance. Shared
  IP users can still hit provider limits; respect bounded Retry-After, expose
  throttling, and never silently retry.
- [Overview/usage terms](https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/index.html)
  restrict promotional assets. Retain only show names, feed URLs, and source
  links from the response; ignore promotional asset fields. Do not fetch/use
  Apple artwork, badges, preview audio, or downloaded media.
  Retain the Apple Podcasts source link for attribution in the future preview.
  These technical docs are not a blanket license for promotional assets or RSS
  media. Access/terms can change; provider stays replaceable behind Search.

## Bounds and privacy

Online queries are explicit submissions, never keystroke telemetry. Normalize
whitespace, cap at 128 characters, apply a 15-second total deadline (including
quota wait), five redirects, a 1 MiB decoded body, and 25 normalized unique shows.
Reject credentials/non-HTTP URLs, redact network/body errors, cache metadata
only. Waiting cancellation does not reserve future quota slots. Preserve a
caller's stricter redirect policy/deadline. No extra dependency is needed.

Cached episode search is separate: read subscribed cached episodes, including
completed history, match plain-text titles/descriptions case-insensitively, cap
at 100 results, and never invoke a network fetcher. It is intentionally not the
uncompleted-only New tab. Existing v1 library reads still materialize the cached
subscribed set; a bounded SQL/full-text index can replace this if scale demands.

## Remaining in #12

Wire `f` and asynchronous/stale-safe results into the TUI; keep `/` local.
Preview feeds before subscribing, provide source attribution, and add ordinary
website alternate-feed discovery with bounded/cancelable HTTP. This foundation
alone does not complete the issue or expose a new user-facing key.
