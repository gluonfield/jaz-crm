# Native company profile fields

- [x] Add Founded year and Size to the built-in Companies schema for existing and new workspaces.
- [x] Preserve existing company records and any fields/values already created.
- [ ] Support the fields in tables, profiles, New/Edit, filters and existing MCP record tools.
- [ ] Display years without thousands separators.
- [ ] Review, verify real PostgreSQL/MCP persistence and actual UI, commit/push and activate locally and on Railway.

Size uses employee ranges; the alternative of exact headcount was offered, and the recommended range interpretation is used while awaiting optional clarification. Founded year uses the existing number type; unknown values remain empty. Both fields reuse native attribute/value storage, existing controls and revision history.

Review: no new field type, tool, dependency or UI component. Whole-year validation stays in canonical value validation; year formatting stays in the existing number formatter. Real PostgreSQL/HTTP/MCP checks cover native discovery through write/read, edit, both filters, rejected fractional/nonfinite/out-of-range years and unknown size ranges, and clearing. The production migration test preserves existing company values and custom size options. Schema, validation and migration negative controls each produce the intended failure.

| Before | After |
| --- | --- |
| Companies need custom attributes for founding year and employee size. | Both are built-in attributes for existing and new workspaces. |
| Generic numeric formatting groups a year as `1,984`. | Founded year displays as `1984`. |
