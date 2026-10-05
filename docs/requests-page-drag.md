# Page dragging and review attribution

- [x] Identify who created RFQ and quotation competitor review from CRM history and the originating conversation.
- [x] Drag sidebar documents onto another page to nest them using the existing parent relationship.
- [x] Make moving a nested page back to the top level discoverable during dragging.
- [x] Preserve navigation/content, prevent cycles and verify save/reload in the browser and build both web/embedded surfaces.
- [x] Review the implementation and run full frontend/backend checks.

The review was created by Codex in the Slotted Lever Stopper CAD for Xometry conversation (20261004T162013-15056ffe), following Augustinas's RFQ/quotation research request on 5 October 2026. The CRM record 775cca63-fc47-462f-8038-81fbfc6a020f was created at 07:43:06 UTC; history attributes the write to Augustinas's authenticated account. No page content or parent was changed during the attribution check.

Sidebar dragging reuses `upsert_record` and the existing parent relationship, tree helpers, query invalidation and server cycle protection. The destination opens after a successful move; the Data heading becomes Move to top level during a drag. Tables and external drags are not page targets. Context-menu moving remains available.

Strict review keeps drag state in one sidebar-owned hook, without a second hierarchy or backend endpoint. Browser verification uses the actual built app and HTTP handlers with a disposable PostgreSQL database: native nesting with descendants, top-level movement, persisted reload, selected-route/content retention, descendant-cycle rejection without a write, failed-request recovery followed by a successful retry, and another drag blocked while a move is pending. The scratch fixture stays outside the repository and its database was removed on completion. Full frontend checks and Go build/vet/tests pass.

| Before | After |
| --- | --- |
| Page movement required the context menu. | Sidebar rows drag onto a destination page; valid destinations highlight and the moved page becomes visible there. |
| Returning to the top level required the menu. | The Data heading becomes a highlighted Move to top level drop target while dragging. |
