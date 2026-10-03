# Independent client UI — 2026-10-03

- [x] Keep search text, active conditions, selected presets and navigation in each client's router state.
- [x] Preserve shared records and named preset definitions.
- [x] Remove active-selection API, persistence and polling; drop obsolete workspace selections during migration.
- [x] Verify independent clients, reload, cleared filters and shared presets; run full checks and review.

The URL owns the active search and filter selection. Clients of the same member and different members act independently. Named preset definitions remain shared workspace data; choosing or editing the active conditions only updates that client's route. Explicit empty conditions stay empty on reload; an unfiltered Follow-ups link opens with the standard Open condition.

Verification: full Go 1.26 tests, build and vet; frontend tests, typecheck, lint, web and embedded-app builds pass. Three production UI instances use real authenticated sessions against a temporary database: two clients of one member and a second member retain independent multiword searches, navigation and preset selections through polling and reload. Explicit empty conditions remain empty after reload; no active-selection requests occur. The migration preserves shared records and named presets, and its regression fails when the table removal is disabled. Saved presets retain schema-deletion cleanup. The maintainability review found no blockers.

Delivery requested: commit and push the verified fix.
