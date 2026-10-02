# Workspace filters — 2026-10-02

- [x] Confirm scope: filters belong to the workspace, not individual users.
- [x] Persist the active filter for each workspace/object, shared across members and devices.
- [x] Preserve named presets, filter links and the follow-up default; allow clearing conditions.
- [x] Verify member sharing, workspace/object isolation, refresh, edits and schema cleanup.
- [x] Run full checks and a maintainability review.

Conditions, text search and the selected preset now belong to the object's workspace. Open sessions refresh every five seconds. Applying or clearing conditions saves immediately; search typing is debounced and flushed on blur. Existing filter links apply once and are consumed so reload cannot resurrect removed conditions. Follow-ups initially show Open; clearing that condition persists.

Verification: Go build, vet and full PostgreSQL-backed suite; frontend typecheck, lint and production builds; two authenticated browser sessions against the real API/database, plus a separate workspace. Browser checks cover immediate navigation, multiword typing, reload, shared clearing, link consumption and failed-save rollback, with light/dark and narrow-screen inspection. Review found no remaining blockers; no dependencies added.

The preceding reply-all request shipped in `06f4f88`: all intended To/Cc recipients are visible and retained when sending.
