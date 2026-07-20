# Switchyard UI mockup notes

The adjacent `switchyard-interactive-mockup.html` is the canonical interactive visual reference for the production Vue interface.

It contains three implemented views:

- Dashboard with summary cards and project cards.
- Project detail with services, logs, health, actions, and resources.
- Port registry with visual ranges and conflict table.

Interactions:

- Use the left navigation to switch Dashboard and Ports.
- Use **Open →** on a project card to open the project-detail view.
- Use **Cmd/Ctrl + K** to open the command palette.

The implementation plan contains the normative design tokens, route structure, component boundaries, responsive behavior, and accessibility requirements. Reuse the visual hierarchy and density, but implement production components rather than copying the mockup as one large Vue component.
