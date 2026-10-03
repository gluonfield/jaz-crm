# Engineering Rules

- Use Go 1.26.
- Use `github.com/lithammer/shortuuid/v4` for new product IDs, with its default alphabet. IDs retain all 128 UUID bits; never truncate them or invent another alphabet. Treat IDs as opaque, case-sensitive strings. Keep provider-owned IDs in their provider's format.
- PostgreSQL UUID storage may retain the same identity; encode/decode at one owning boundary when exposing shortuuid IDs. Preserve existing data and links when changing ID presentation. Security tokens still use cryptographically secure token generation.
- Write self-documenting code. Add comments only to explain non-obvious behavior, constraints or reasons the code cannot express; omit comments that narrate the code.
- Keep implementations and JSON minimal; every line and field must earn its place. Prefer correcting contracts and deleting duplication over adding branches or layers.

- Never add useless or fake tests. Every test must run in the normal verification path, exercise real production behavior, and fail when that behavior breaks. Delete tautologies, tests of trivial helpers or forwarding, checks of source text or internal constants, duplicate coverage, and permanently skipped or inactive tests.
- Fakes may isolate external boundaries; they must preserve the real contract and never stand in for end-to-end verification.

## Jaz design language

Linear is the reference and Jaz Tasks its in-house model; CRM and future products match both.

- Make aesthetic, product and UI/UX decisions as a world-class product designer and product manager. Start from the established, well-tested patterns of leading products for the same job, such as chat threads, mail readers, composers, lists and navigation, and adapt them to Jaz instead of inventing new interactions. Judge every screen in its idle and busy states: each element earns its space at the moment it appears.

- Calm, dense work surfaces. Records set the hierarchy; no filler copy, explanatory paragraphs or decorative UI.
- Colour comes only from the tokens in `frontend/src/styles.css` (see `frontend/THEMING.md`): `bg` content, `panel` chrome, `raised` menus and dialogs, `ink`/`ink-2`/`ink-3` text, `border` hairlines, `primary` for selection and primary actions with white labels. Support light, dark and MCP host themes.
- Inter 13px for UI, 12px for secondary text, medium-weight headings, tabular figures for counts. Icons are 16px in `ink-2`.
- `--radius-control` (6px) for controls and rows, `--radius-card` (10px) for cards, menus and popovers. Separate surfaces by fill, not outlines; only floating layers get shadows.
- Actions use `Button` from `@jaz/ui/button`, the source package in Jaz Tasks `ui/`. Both products pin the same Git commit. Choose `variant` and `size`; callers add layout only. Keep form submission explicit with `type="submit"`; update the shared source instead of copying styles or adding another Button.
- Tasks' 232px sidebar, slim page headers, aligned table rows and short board cards. Create and edit in dialogs or contextual controls.
- Menus share `components/ui/menu.tsx`, `context-menu.tsx` and `dropdown-menu.tsx`, kept identical in every product: 32px rows of icon and label, `ink-3` shortcut hints where a shortcut exists, a filled ▸ on submenus, edge-to-edge separators. Group properties, then actions, then delete; delete stays neutral and confirms. Property submenus use `ContextMenuOptions`: a filter field, a check on the current value and number keys 1–9.
- Right-click menus repeat visible controls, never replace them. Hover and selection fills stay subtle; focus stays visible.
- Motion is 100–160ms on named properties, interruptible, and off under reduced motion. No bounce or load choreography.
- Inline MCP results fit their content; full app views keep navigation and record links.
- Verify UI in the real screen with realistic data, both themes and a narrow width, against Tasks and Linear.
