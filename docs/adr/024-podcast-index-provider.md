# ADR 024: Optional authenticated Podcast Index search

Status: implemented with deterministic fixtures; live authorized account smoke pending.

Research official API docs and Podcastindex-org/legal TermsOfService.md before
implementation (links/setup/limitations in ../podcast-index.md). Numeric service
quotas are not guaranteed by terms; use conservative local bounds. User supplies
own key/secret through environment, never embedded credentials. No blanket legal
compliance/endorsement or real credential verification is claimed.

A small Searcher interface is shared by Apple/Podcast Index; charts remain Apple
only and RSS remains an independent explicit preview action. The scope selector
cycles offline Library → Apple → Podcast Index. Source/query labels remain visible;
requests are captured/generation-bound, never silently retried on another provider.

Index uses signed headers (documented SHA-1 protocol), rejects redirects and keeps
no response cache. Reuse Apple's bounded JSON I/O, quota gate and result URL/title
normalization without using its response cache. API results and selected feed URLs
are current-view state only: do not persist them on provider exit or app restart.
Persist only navigation metadata (provider/country/query) for restoring the Discover view;
requery Index on entry. Any future response cache must honor provider cache headers.
The migration removes legacy Index directory rows persisted by earlier builds.
Persist only independently fetched RSS metadata after explicit
playback/enqueue/subscription; cross-provider episode identity is
feed/GUID/enclosure based, not directory ID. Source links retain Index attribution.
External mpv receives no Podcast Index credential environment values.

No new dependency. Fixture tests exercise auth/redaction, limits/cancellation,
missing access, provider lifecycle, read-only preview and stable ad-hoc identities.
Live provider and real terminal/native/font checks remain separate acceptance.
