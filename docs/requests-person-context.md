# Native People Context

- [x] Context belongs to the built-in People schema in existing and new workspaces.
- [x] Preserve existing records and any saved Context values.
- [x] Show a prominent multiline Context editor on the person profile and support it in New/Edit.
- [x] Expose Context through MCP schema discovery, reads, writes, search and change history.
- [x] Review, verify schema/MCP persistence and profile/New UI behavior, commit/push and activate locally and on Railway.
- [x] Complete Edit-save and light/dark narrow-profile acceptance after the Jaz side browser reconnects.
- [ ] Complete native keyboard acceptance; the browser transport currently delivers no key events.
- [x] Review existing support and fix New/Edit and profile overflow when Context contains a long unbroken URL.

Review: reuse native attribute/value storage and its revision history; no custom-field setup, new tool, dependency or second copy of Context. Profile editing saves on blur or Cmd/Ctrl+Enter, Enter adds a newline and Escape discards the draft. New/Edit use a multiline input and inline MCP cards show a three-line preview.

Production PostgreSQL/MCP regressions pass for fresh schema, old-schema migration, preservation, multiline reads/edits, text search, revision history and clearing. Both regressions fail with their schema change disabled, then pass with it restored.

Delivery: implementation `06b7256` is on main. GitHub verification and server/worker image publication succeeded. Railway Server and Worker report SUCCESS on descendant `4d07a5e`; local services run that exact published revision and both health endpoints return 200. All three local People schemas have native text Context; migration 16 is applied.

Actual CAS browser acceptance: New creates multiline Context; profile edits persist after reload, clearing removes the stored value, and the dark screen was visually reviewed. Edit displays its multiline input. Escape's React handler restores the saved text, but the browser transport delivered no native key events, so Enter/Cmd+Enter/Escape keyboard acceptance remains unverified. The browser disconnected before Edit-save and light/narrow checks; the temporary person was deleted through MCP, with no customer records changed.

| Before | After |
| --- | --- |
| People have no standard relationship-background field. | Context is native in every People schema and editable directly below the profile, in New/Edit, and through existing MCP record operations. |

Review on 2026-10-01 confirmed existing native support and passing fresh PostgreSQL/MCP regressions. Long unbroken Context exposed the grid items' automatic minimum width: the 620px Edit dialog held a 1515px form, moving Save beyond a 784px viewport; the profile editor also extended beyond that viewport. Applying `min-width: 0` at each owning grid item restored the form to 618px and kept the profile editor inside the content column, with no horizontal overflow. The fix preserves multiline content and automatic height.

Review fix `e079422` is pushed and both Railway deployments report SUCCESS. Edit-save preserved three lines including a long URL. The actual deployed profile was visually inspected in light and dark at 640px: Context stays within the 352px content column without horizontal overflow, using the production CSS with no override. Local services now run published descendant `7171864`, with both health checks passing. No customer values were changed.

| Before | After |
| --- | --- |
| New/Edit's form grows wider than its dialog for long Context. | `create-record.tsx` constrains the form's minimum width with `min-w-0`, keeping Save visible. |
| The profile summary's grid item grows beyond its content column. | `r.$recordId.tsx` uses `min-w-0` on that grid item so Context wraps within the profile. |
