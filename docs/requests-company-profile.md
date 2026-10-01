# Native company profile fields

- [x] Add Founded year and Size to the built-in Companies schema for existing and new workspaces.
- [x] Preserve existing company records and any fields/values already created.
- [x] Support the fields in tables, profiles, New/Edit, filters and existing MCP record tools.
- [x] Display years without thousands separators.
- [x] Review, verify real PostgreSQL/MCP persistence and actual UI, commit/push and activate locally and on Railway.

Size uses employee ranges; the alternative of exact headcount was offered, and the recommended range interpretation is used while awaiting optional clarification. Founded year uses the existing number type; unknown values remain empty. Both fields reuse native attribute/value storage, existing controls and revision history.

Review: no new field type, tool, dependency or UI component. Whole-year validation stays in canonical value validation; year formatting stays in the existing number formatter. Real PostgreSQL/HTTP/MCP checks cover native discovery through write/read, edit, both filters, rejected fractional/nonfinite/out-of-range years and unknown size ranges, and clearing. The production migration test preserves existing company values and custom size options. Schema, validation and migration negative controls each produce the intended failure.

| Before | After |
| --- | --- |
| Companies need custom attributes for founding year and employee size. | Both are built-in attributes for existing and new workspaces. |
| Generic numeric formatting groups a year as `1,984`. | Founded year displays as `1984`. |

Delivery: `7171864` is pushed. Full frontend checks, Go build/vet/full suite and all three negative controls pass. GitHub verification and both image publications succeeded; exact-revision Railway Server/Worker deployments report SUCCESS. Local services run those published images; migration 17 adds the fields to all three Companies schemas and both health endpoints return 200. Authenticated CAS schema discovery confirms both native fields without customer mutations.

Actual deployed UI: New exposes the year input and range picker; creating a scratch company populates table columns, profile edits persist both fields, and Edit-save persists the changed year. The dark Edit dialog was visually inspected with Save inside the viewport and no inline layout override; profile/table years render without grouping. The browser disconnected before the final visual filter flow. Both fields' combined filters pass through production PostgreSQL/MCP; no additional filter implementation is required.

The scratch workspace was deleted through the production MCP tool; the membership list contains only Personal and CAS, with CAS still the connection default. No scratch company or customer mutation remains.
