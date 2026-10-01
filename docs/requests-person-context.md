# Native People Context

- [x] Context belongs to the built-in People schema in existing and new workspaces.
- [x] Preserve existing records and any saved Context values.
- [x] Show a prominent multiline Context editor on the person profile and support it in New/Edit.
- [x] Expose Context through MCP schema discovery, reads, writes, search and change history.
- [ ] Review, verify real persistence and UI behavior, commit/push and activate locally and on Railway.

Review: reuse native attribute/value storage and its revision history; no custom-field setup, new tool, dependency or second copy of Context. Profile editing saves on blur or Cmd/Ctrl+Enter, Enter adds a newline and Escape discards the draft. New/Edit use a multiline input and inline MCP cards show a three-line preview.

Production PostgreSQL/MCP regressions pass for fresh schema, old-schema migration, preservation, multiline reads/edits, text search, revision history and clearing. Both regressions fail with their schema change disabled, then pass with it restored.
