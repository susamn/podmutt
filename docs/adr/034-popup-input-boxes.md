# ADR 034: Styled input boxes and centered popup overlays

Status: implemented for #37.

Search, URL entry, feed subscription, and confirmation popups adopt mpdtui's
focused input modal structure:

1. **Overlay Geometry**:
   - Centered on the terminal viewport using Unicode box-drawing borders.
   - Sized to fit content and input width with strict height caps to avoid displacing margins.

2. **Dedicated Input Box**:
   - Renders a visually distinct typing field with `Selection` background color.
   - Displays a trailing solid cursor `▌`.
   - Labels clearly identify field purpose (`Query: `, `URL: `).

3. **Footer Actions**:
   - Clean shortcuts footer displayed at the bottom of the modal:
     `Enter submit · Tab switch source · Esc cancel · Ctrl+U clear`.
