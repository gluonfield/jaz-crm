# Tables Inside Pages

- [x] Create a Page or Table inside a page from its content and sidebar controls.
- [x] Show child tables alongside child pages in the sidebar and as links on the parent page.
- [x] Move existing tables inside a page or back to the top level.
- [x] Mention and open table links from page content.
- [x] Persist nesting through the shared API/MCP contract, preserving workspace isolation and page deletion/restoration.
- [x] Run full checks and strict maintainability review.
- [ ] Verify the real UI: the integrated browser reports `Side browser did not open` through both navigation tools despite reporting connected. Visual and interaction verification remains open.

Source: Augustinas, 2026-10-13.

Tables keep their existing object identity and gain an optional parent page; creation is shared by root and child controls. Moving uses `edit_object` with `action: move`; omitting `parent` returns to the root. A trashed parent leaves its table accessible at the root, retaining the relationship for restoration. Built-in CRM objects stay in their existing navigation.

| Before | After |
| --- | --- |
| Root-only Table creation; child controls created a page directly. | Root, page-body and sidebar + menus offer Page or Table; page context menus offer both. |
| Pages listed only child pages. | Pages link to child tables and pages in the sidebar's order; nested tables show their ancestors and open their branch. |
| Tables could only be renamed or deleted. | Move to places a table under a page or back at the root. |
| Mentions and internal navigation targeted records. | @ also finds tables by name or parent path, inserts table links with table icons, and opens them internally. |
| Schema/API had no table nesting. | PostgreSQL, API and MCP carry the same parent relationship. |

Verification: all 18 frontend tests, typecheck, lint, web and embedded builds pass; full Go tests, build and vet pass. Real HTTP handlers against isolated PostgreSQL confirm nested creation, existing-table movement, rows, stored table links and returning to the root. The normal MCP test covers invalid/foreign/trashed parents, workspace isolation, built-in-object protection and restoration; disabling parent-type validation makes it fail. Strict review found no new dependencies, duplicated hierarchy storage or cross-layer contract leaks.
