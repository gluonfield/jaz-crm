# Record Deletion and Trash

Source: Augustinas, 2026-10-08.

- [x] Recreate Company → Videos with the three supplied video links and both PDF labels (`LtyxMFUPSnjkkZiMk5JHcZ`).
- [ ] Recover the two original PDF destinations; URLs requested from Augustinas. No database backup or PITR is available.
- [x] All record deletion entry points show a confirmation modal.
- [x] Confirmed record deletion moves records to Trash with their content retained.
- [x] Show Trash in the sidebar when it contains records; allow restoration.
- [x] Preserve workspace isolation, page hierarchy and existing email triage semantics.
- [x] Verify real UI cancellation, deletion, reload and restoration; run full checks and strict review.
- [x] Commit, push and verify deployment and served UI assets.

Recovery comes first. Inspect current records and available snapshots before reconstructing supplied content. Do not roll back the shared production database.

Trash retains the original IDs, values, history, references and conversation links. Children remain available at top level while their parent is deleted, and reconnect when it is restored. Newer moves and triage decisions are preserved; explicitly restoring contacts returns deletion exclusions to Kept. Tables and columns retain their existing permanent-delete confirmations.

Real PostgreSQL-backed browser checks pass for the header, sidebar and record-row confirmation paths, cancellation without a write, single confirmed deletion, reload, restoration, child nesting, conditional Trash visibility and light/dark/760px presentation. Regression checks exercise MCP workspace isolation, populated migration, history, cycle prevention, retained email content, later manual decisions and company-only domain restoration. A negative control confirms the webmail restoration check fails when the company-domain policy is bypassed. Full frontend and backend checks, builds, vet and strict review pass.

Published `8703a4cb8fa0408c19bdb25489abfd9069a115a8` to main. Server and Worker both report SUCCESS for that commit; health and the CI verification job pass. The served Trash route and stylesheet exactly match the browser-verified build. Videos content and nesting were read back in production. The side browser disconnected before the final deployed-modal check; no production deletion was used for verification. The two PDF URLs remain the only unresolved recovery item.
