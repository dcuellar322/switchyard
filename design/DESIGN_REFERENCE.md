# Switchyard design reference

## Files

- `switchyard-interactive-mockup.html` — canonical interactive design reference.

Open the HTML directly in a browser. It includes three primary views:

1. Dashboard
2. Project detail
3. Port registry

Use the in-page navigation and project **Open** actions to inspect each view. Use `Cmd/Ctrl+K` to inspect the command palette.

## Implementation guidance

- Reproduce the visual hierarchy, dimensions, tokens, status treatments, and density.
- Replace mock data with generated API clients and real queries.
- Do not copy the single-file mockup architecture into production.
- Break the Vue implementation into domain-focused views and cohesive components.
- Preserve accessibility, responsive behavior, and all loading/error states.
- Create production Playwright visual baselines at 1440×1050 after the Vue implementation is approved.

## Key tokens

```css
--bg: #0a0d12;
--panel: #11161e;
--panel-2: #151b24;
--panel-3: #1a2230;
--border: #253044;
--text: #edf3fb;
--muted: #91a0b5;
--soft: #65748a;
--accent: #78a6ff;
--accent-2: #9e7bff;
--green: #54d49a;
--yellow: #f1c75b;
--red: #ff7373;
--cyan: #63d7e7;
```
