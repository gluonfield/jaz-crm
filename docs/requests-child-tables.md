# Tables Inside Pages

- [x] Create a Page or Table inside a page from its content and sidebar controls.
- [x] Show child tables alongside child pages in the sidebar and as links on the parent page.
- [x] Move existing tables inside a page or back to the top level.
- [x] Mention and open table links from page content.
- [x] Persist nesting through the shared API/MCP contract, preserving workspace isolation and page deletion/restoration.
- [x] Run full checks and strict maintainability review.
- [x] Verify the real UI in the integrated browser against isolated PostgreSQL.

Source: Augustinas, 2026-10-13.

Follow-up from the supplied screenshots:

- [x] Show Add Page and Add Database as visible page-body actions.
- [x] Add right-click actions to child page and database links in the page body.
- [x] Inspect the deployed version and verify actual browser interactions after the screenshot showed the old Add page control.
- [x] Databases use the page icon picker, with icons reflected in navigation, child links and mentions.
- [x] Improve the database surface using the supplied Notion reference: editable title/icon, optional description and a distinct view/table toolbar.
- [x] Drag databases onto pages and back to the top level, using the same interaction as pages.
- [x] Verify real database drag/drop, saved nesting after reload, row preservation and existing page-drag guards.
- [x] Complete the requested thermo-nuclear review of database dragging before publication.

Tables keep their existing object identity and gain an optional parent page; creation is shared by root and child controls. Moving uses `edit_object` with `action: move`; an empty `parent` returns to the root and an omitted one keeps the current parent. A trashed parent leaves its table accessible at the root, retaining the relationship for restoration. Built-in CRM objects stay in their existing navigation.

| Before | After |
| --- | --- |
| Root-only Table creation; child controls created a page directly. | Root, page-body and sidebar + menus offer Page or Table; page context menus offer both. |
| Pages listed only child pages. | Pages link to child tables and pages in the sidebar's order; nested tables show their ancestors and open their branch. |
| Tables could only be renamed or deleted. | Move to places a table under a page or back at the root. |
| Mentions and internal navigation targeted records. | @ also finds tables by name or parent path, inserts table links with table icons, and opens them internally. |
| Schema/API had no table nesting. | PostgreSQL, API and MCP carry the same parent relationship. |
| Child links in page content had no right-click actions. | Shared page/database menus work in the body and sidebar, with Open, Rename, Move to and Delete; pages also create children. |
| The page-body creation choice was behind Add inside. | Add Page and Add Database are both visible; sidebar creation menus use Database. |
| Databases had a fixed table glyph and a small header title. | The page icon picker changes a saved database icon; a prominent editable name and optional description sit above the toolbar. |
| Custom rows used coloured initials. | Custom database rows use document icons, with true record counts retained in the view control. |

Verification: all 18 frontend tests, typecheck, lint, web and embedded builds pass; full Go tests, build and vet pass. Real HTTP handlers against isolated PostgreSQL confirm nested creation, existing-table movement, rows, stored table links and returning to the root. The normal MCP test covers invalid/foreign/trashed parents, workspace isolation, built-in-object protection and restoration; disabling parent-type validation makes it fail. Strict review found no new dependencies, duplicated hierarchy storage or cross-layer contract leaks.

Follow-up verification: native right-click on both child-link kinds opens the shared menus; Rename updates both links; Add Database creates a nested database with its initial row. Choosing a green Phone icon persists to PostgreSQL and appears in the sidebar, breadcrumbs, header and parent link after reload. Description autosave preserves focus in the name; Enter saves and Escape cancels title edits. Light/dark renders and a measured 390px iframe show no document overflow. The picker implementation is shared with pages, metadata writes update only the requested fields, and input drafts stay with their individual controls. The existing release `0718e5e` was confirmed SUCCESS on Railway; publication provenance for the follow-up is recorded in session memory.

Database drag correction: root and nested database rows use the same controller as pages, with existing `edit_object` moves, shared destination feedback and pending guards. The normal frontend rendering regression checks both copies and fails when database handlers are removed. Real browser drag events verify nested-to-page, nested-to-root and root-to-page moves, persisted reload, unchanged navigation/content, failed-save recovery and blocked concurrent drags. Page movement still works and descendant cycles remain rejected. HTTP readback confirms database icon/description and existing record identity/content survive the moves. All 19 frontend tests, typecheck, lint, builds and full Go checks pass. The requested strict review found no further blockers: one shared gesture owner, existing domain shapes and mutation contracts, no new dependencies, hierarchy state or backend endpoints.
