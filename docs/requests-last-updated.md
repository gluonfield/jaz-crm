# Record last updated

- [x] Review the current CRM for an automatic record-level Last updated concept.
- [x] Persist and backfill a timestamp for actual record value changes, including document revisions and removals.
- [x] Expose the timestamp through record tools, record pages and tables, with a latest-updated sort.
- [x] Verify migrations, mutation semantics, API output, sorting and the rendered interface; commit and push completed work.

Review: current records expose created_at and dated value history, but no updated_at. The checkout is gluonfield/jaz-crm; the supplied ../../ink-crm path is absent. Work uses an isolated checkout because the main checkout contains another session's changes.

Validation: full backend build, vet and tests; frontend tests, typecheck, lint and web/embedded builds pass. Real PostgreSQL tests cover historical backfill, actual/no-op writes, removals, coalesced documents, stale writes, MCP serialization and sorted pagination. Side-browser checks cover sort/reload, timestamp refresh after an actual edit, light/dark rendering and narrow-screen access. Preview records use a disposable database.

Historical timestamps use the latest recorded value start/end or creation time. Exact times of past coalesced document edits were not recorded; subsequent revisions update the timestamp precisely.
