# Page Editing

Source: Augustinas, 2026-10-08; Superagent screenshot `15fee0d31b0f8a174b4be1ca1de53765`.

- [x] Plain document links with record icons; external links with chain icons and descriptive underlined labels.
- [x] Clickable Markdown checklists, including checked styling, inline formatting and nested items.
- [x] `---` renders a divider when loading Markdown and typing in the editor.
- [x] Text-only breadcrumbs above the document, with clickable ancestors and the current page last.
- [x] Icon color dropdown shows swatches only, with accessible names and a selected outline.
- [x] Verify editing, saving and reload through the real app; light/dark themes and narrow layout.
- [x] Strict maintainability review and full checks.

Publication provenance (commit/push and exact production deployment) is recorded in the session note `sources/agent/2026-10-08-crm-page-editing` after shipping.

Keep existing Markdown storage and page hierarchy. Preserve URL-only page images; this request adds no uploads.

The shared DocumentKit now supplies the Markdown, table and nested task extensions to the page editor and read-only view. Checked styling applies to the item's own paragraphs; ordinary bullets inside task lists retain their list layout. Links use the shared theme's accent, with a readable dark-mode text variant and visible focus/hover states. Pages have text-only breadcrumbs inside the document column, ending with the current page, with chevron separators, ellipsis with full-title tooltips and a Pages link at the top level. The icon color dropdown is a ten-swatch palette; color names remain in accessible labels and hover tooltips.

Augustinas also asked for supply-chain caution and fewer imports. No new dependencies or lockfile changes; this reuses installed Tiptap and Lucide and removes duplicated extension imports/configuration.

Verification: the normal frontend suite covers real Markdown parsing, nested schema validation, a checkbox edit and stable reserialization/reload. Full frontend tests/typecheck/lint/web+MCP builds and Go tests/build/vet pass. The real authenticated app with isolated PostgreSQL confirms checkbox autosave/reload, internal document navigation, external new-tab links, ancestor navigation, two loaded dividers, light/dark screenshots and a 760px iframe with long ancestors and no content/breadcrumb overflow. The real editor input handlers turn `---` into a divider and `- [ ] ` into a task. Native keyboard automation delivered no events, so keyboard verification uses synthetic DOM Enter and the editor's actual text-input handlers.

Strict review: canonical document configuration removes duplication; no new storage, transport, dependencies or upload paths. Scope remains page presentation and the shared document schema.

Follow-up verification: the swatch menu has ten accessible options, no visible text, a selected outline and 40px hit areas. A native mouse press/release with a frame between them recolors the saved icon, closes the menu and preserves the picker; the production storage adapter confirms the saved color and reload retains it. Root and nested breadcrumbs include the current page, keep ancestor navigation and contain only chevron separators. Light/dark screenshots and a 760px iframe with long titles confirm truncation, full-title tooltips and no breadcrumb or palette overflow. Synthetic ArrowDown exercises the existing Radix menu focus behavior. Full frontend and backend checks pass. Strict follow-up review found no new state, duplicated mechanisms or architecture regressions.
