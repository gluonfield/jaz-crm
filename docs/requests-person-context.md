# Native People Context

- [x] Context belongs to the built-in People schema in existing and new workspaces.
- [x] Preserve existing records and any saved Context values.
- [x] Show a prominent multiline Context editor on the person profile and support it in New/Edit.
- [x] Expose Context through MCP schema discovery, reads, writes, search and change history.
- [x] Review, verify schema/MCP persistence and profile/New UI behavior, commit/push and activate locally and on Railway.
- [ ] Complete Edit-save, light/narrow and native keyboard acceptance when the Jaz side browser reconnects.

Review: reuse native attribute/value storage and its revision history; no custom-field setup, new tool, dependency or second copy of Context. Profile editing saves on blur or Cmd/Ctrl+Enter, Enter adds a newline and Escape discards the draft. New/Edit use a multiline input and inline MCP cards show a three-line preview.

Production PostgreSQL/MCP regressions pass for fresh schema, old-schema migration, preservation, multiline reads/edits, text search, revision history and clearing. Both regressions fail with their schema change disabled, then pass with it restored.

Delivery: implementation `06b7256` is on main. GitHub verification and server/worker image publication succeeded. Railway Server and Worker report SUCCESS on descendant `4d07a5e`; local services run that exact published revision and both health endpoints return 200. All three local People schemas have native text Context; migration 16 is applied.

Actual CAS browser acceptance: New creates multiline Context; profile edits persist after reload, clearing removes the stored value, and the dark screen was visually reviewed. Edit displays its multiline input. Escape's React handler restores the saved text, but the browser transport delivered no native key events, so Enter/Cmd+Enter/Escape keyboard acceptance remains unverified. The browser disconnected before Edit-save and light/narrow checks; the temporary person was deleted through MCP, with no customer records changed.

| Before | After |
| --- | --- |
| People have no standard relationship-background field. | Context is native in every People schema and editable directly below the profile, in New/Edit, and through existing MCP record operations. |
