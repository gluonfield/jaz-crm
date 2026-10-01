# Engineering Rules

- Use Go 1.26.
- Use `github.com/lithammer/shortuuid/v4` for new product IDs, with its default alphabet. IDs retain all 128 UUID bits; never truncate them or invent another alphabet. Treat IDs as opaque, case-sensitive strings. Keep provider-owned IDs in their provider's format.
- PostgreSQL UUID storage may retain the same identity; encode/decode at one owning boundary when exposing shortuuid IDs. Preserve existing data and links when changing ID presentation. Security tokens still use cryptographically secure token generation.
- Write self-documenting code. Add comments only to explain non-obvious behavior, constraints or reasons the code cannot express; omit comments that narrate the code.
- Keep implementations and JSON minimal; every line and field must earn its place. Prefer correcting contracts and deleting duplication over adding branches or layers.

- Never add useless or fake tests. Every test must run in the normal verification path, exercise real production behavior, and fail when that behavior breaks. Delete tautologies, tests of trivial helpers or forwarding, checks of source text or internal constants, duplicate coverage, and permanently skipped or inactive tests.
- Fakes may isolate external boundaries; they must preserve the real contract and never stand in for end-to-end verification.

## Jaz design language

All Jaz products share this language. Jaz Tasks is the reference for layout, density, typography and interaction; CRM and future products should extend it with domain-specific content.

- Build a calm, compact work surface inspired by Linear. Let the user's records and actions establish the hierarchy. Keep labels concise and remove filler, redundant instructions and decorative UI.
- Reuse existing components and semantic tokens from `frontend/src/styles.css`; follow `frontend/THEMING.md`. Share token names and meanings across products. Support light/dark schemes and MCP host theme variables, fonts, radii and shadows.
- Use Inter, 13px base UI text, 12px secondary labels and restrained medium-weight headings. Keep icons around 16px, optically aligned with labels. Use monospace for code and tabular figures for changing counts.
- Separate surfaces with subtle fills and spacing: `bg` for content, `panel` for chrome, `surface`/`raised` for elevated layers, and `ink`/`ink-2`/`ink-3` for text hierarchy. Use quiet hairlines only where separation needs them; reserve soft shadows for menus, dialogs and dragged items.
- Follow Tasks' compact sidebar (232px desktop reference), slim page headers, aligned table rows and concise board cards. Use the same spacing rhythm and column alignment across screens. Keep creation and editing in focused contextual controls or dialogs.
- Use a restrained primary accent for primary actions and selection, with white labels on solid primary buttons. Give secondary actions quiet neutral styles. Use status colours to convey actual state; preserve workspace-defined colours.
- Default to 6px control radii and 10px card radii through the shared tokens. Keep nested corners visually concentric. Avoid oversized buttons and unnecessary bordered containers.
- Keep row hover and selection subtle. Offer clear keyboard focus, accessible names, keyboard navigation and visible touch actions. Right-click menus complement discoverable controls.
- Use short, interruptible transitions, usually 100–160ms, for hover, menus and small state changes. Preserve layout geometry, respect reduced motion and animate only named properties. Use movement to explain a change, without decorative bounce or page-load choreography.
- Keep standalone and embedded surfaces consistent. Inline MCP results should fit their content; full app views retain navigation and record links.
- Verify UI changes in the real screen with realistic content, both themes and a narrow/touch layout. Compare against Tasks before introducing a new visual pattern.
