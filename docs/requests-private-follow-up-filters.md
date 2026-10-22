# Private Follow Up Filters

Source: Augustinas, 2026-10-22.

- [x] Everyone and Assigned to me are fixed built-in views; Assigned to me remains the default.
- [x] Every other Follow Ups saved filter belongs to its authenticated creator, including MCP writes.
- [x] Enforce privacy on listing, updating and deleting, including guessed IDs and same-name filters.
- [x] Preserve existing presets as independent private copies for current members.
- [x] Verify migration, MCP, UI and full normal checks; complete thermo-nuclear review.
- [x] Commit and push verified work.

Implementation: two backend-defined fixed views; creator-scoped Follow Ups filters and per-owner names; other objects retain shared presets. Legacy presets become independent copies for current members, with the earliest member retaining the original ID. Reserved built-in names are disambiguated when preserving old custom presets. Removed the separate owner picker.

Review fixed deletion callback cancellation during cache refresh, retained the confirmation target through closing, and unified picker/count view data. Full Go 1.26 tests/build/vet, frontend tests/typecheck/lint/build and all three normal browser suites pass. Browser coverage includes real PostgreSQL-backed save/reload/delete, light and narrow dark layouts, thirteen workflow scenarios and the embedded MCP app. Four negative controls fail for the intended list/update/delete/migration reasons. Verification artifacts are in ignored `runs/`; no live CRM records or provider calls were used.
