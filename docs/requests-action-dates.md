# Action dates and Chase

- [x] Visible Chase preset for existing and new workspaces: open, waiting on Them, overdue; chronological ordering.
- [x] Replace review_on with an optional action_date that preserves date-only or timestamp precision; migrate values and saved/active filters.
- [x] Workspace timezone and prompt anchors for relative dates; stated/suggested provenance with reason and source.
- [x] Prompt examples for no action, end of week, after a call, explicit deadlines, next month and unfinished overdue actions.
- [x] Enforce no automatic Them drafts; withdraw stale AI drafts and retain human edits.
- [x] Atomic clearing/completion and manual-date protection, including explicit clears; retries retain dates.
- [x] Display and editing of date/time and provenance, sorting/filtering across pagination and daylight saving.
- [x] Behavioral and migration tests, real-screen checks, strict review and full checks.
- [x] Commit/push and production verification.

Scope approved after specification on 2026-10-03. Automatic chase drafting remains outside this change.

Validation: full PostgreSQL-backed backend suite, build and vet; frontend date/DST tests, typecheck, lint and both builds; real web and embedded screens in light/dark at 1180 and 768 pixels. Tested Chase membership and order, optional time editing, source disclosure and persisted manual clears. Seven live gpt-6-luna drafting examples pass. The live probe exposed an offset error across the London clock change; the planner now returns local time and Go applies the timezone.

Review: date precision is a schema type; query sorting/filtering uses one deadline conversion. Existing record-write transactions enforce completion, source precedence and draft rules. Manual dates use the existing explicit-override convention used by the composer; the automatic drafting agent retains only its read-only tools. No dependencies added. Old dates retain their value and audit history; agent-authored legacy dates are Suggested because their original rationale was not recorded.

Production: `3682736` is pushed and active on Server and Worker. Health, migrated Chase conditions, London timezone, preserved knowledge settings and web-access setting pass. All 11 active dates migrated; four closed dates remain only in history. The questioned October 19 date is preserved as Suggested with the honest legacy reason. No stale automatic drafts remain on Them actions. The deployed embedded resource matches the tested build after its server-injected MCP URL; the website date controls match after chunk hashes.

## Filter correction, 2026-10-03

- [x] Put Needs attention before Chase, followed by Waiting on them.
- [x] Include open actions waiting on Us in Needs attention regardless of date; migrate unchanged existing presets and selected filters while preserving custom filters.
- [x] Verify future and undated Us actions remain visible and Them actions use their own queues, including a fixture matching the reported item.

The reported October 19 item was Open and waiting on Us. The former Needs attention deadline condition excluded it, while both Them presets correctly excluded it by direction.

Validation: full backend build/vet/PostgreSQL tests and frontend checks pass. Real web/embedded filter selection passes in light/dark and at narrow width, including persistence across reload. Reverting either preset membership or ordering makes the API regression fail. Migration checks cover unchanged, edited, renamed and unrelated presets. Strict review keeps ordering in the record service, reuses schema order, and changes the filter contract without adding date exceptions to the UI.

## Thermo-nuclear review, 2026-10-03

- [x] Audit the filter correction, its ownership, ordering and migrations.
- [x] Close the reproduced gap where an optional, unset Waiting on field excludes open work from every preset; preserve unset ownership and custom filters.
- [x] Verify the corrected contract through API, migration and real-screen checks.

Finding: the new predicate assumed every open action had a waiting side. Creation permits omission, and the earlier migration test was changed to supply Us, masking the regression. Needs attention now uses Open + Waiting on is not Them, which includes unset values through the existing filter engine. Built-in ordering already has one owner and reuses schema order; no additional ordering abstraction is warranted.

Verification: the original predicate fails the real API reproduction. Full backend build/vet/PostgreSQL tests and frontend checks pass after correction. Real website and embedded views pass for dated and undated unassigned actions in both themes and at narrow width; preset order, Them queues and reload persistence remain correct. Migration coverage includes both prior default shapes and edited/custom selections.
