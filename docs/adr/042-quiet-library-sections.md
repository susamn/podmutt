# ADR 042: Quiet separation of library views and subscriptions

Status: accepted from user review; extends ADR 040.

Keep one sidebar. Place a muted Subscriptions label with a trailing hairline
between the three library views and individual shows. Use the existing background
and selection style, without another box or icons. Omit the blank spacer when
the viewport has fewer than six list rows. Hide the caption when no shows match;
search/discovery result lists remain ungrouped.

The caption/spacer are rendering-only rows, never items or focus stops. Rendering
and paging share their physical-row mapping so selection stays visible across
scrolling, filtering, refresh, and compact layouts.

Shorten All subscriptions to All episodes in the sidebar; retain the explicit
All subscribed episodes heading in Episodes. Filtering, scope, IDs, subscription,
and playback semantics do not change. No new dependency or storage changes.
