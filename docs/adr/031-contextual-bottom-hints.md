# ADR 031: Contextual highlighted bottom controls hint bar

Status: implemented for #34.

Inline navigation keys previously shown in the Podcasts list pane are removed, giving
the podcast catalog full vertical height.

Instead, controls are shown in the bottommost hint bar matching mpdtui's pattern:
- Formatted as `key:action` pairs with bold accent-highlighted keys (`Cyan`) and
  plain action descriptions.
- Contextual per active page and focus:
  - **My subscriptions**: `f:search`, `U:url`, `b:discover`, `v:view`, `o:sort`, `c:complete`, `Space:play`, `,/.:seek`, etc.
  - **Discover**: `f:search`, `U:url`, `b:library`, `t:top25`, `Space:play`, `a:enqueue`, `u:subscribe`, `R:retry`, `,/.:seek`, etc.
  - **Queue / Details / Player**: specific focus actions.
- Transient notices are prepended cleanly without crowding out keybindings.
