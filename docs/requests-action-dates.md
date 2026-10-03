# Action dates and Chase

- [x] Visible Chase preset for existing and new workspaces: open, waiting on Them, overdue; chronological ordering.
- [x] Replace review_on with an optional action_date that preserves date-only or timestamp precision; migrate values and saved/active filters.
- [x] Workspace timezone and prompt anchors for relative dates; stated/suggested provenance with reason and source.
- [x] Prompt examples for no action, end of week, after a call, explicit deadlines, next month and unfinished overdue actions.
- [x] Enforce no automatic Them drafts; withdraw stale AI drafts and retain human edits.
- [x] Atomic clearing/completion and manual-date protection, including explicit clears; retries retain dates.
- [x] Display and editing of date/time and provenance, sorting/filtering across pagination and daylight saving.
- [x] Behavioral and migration tests, real-screen checks, strict review and full checks.
- [ ] Commit/push and production verification.

Scope approved after specification on 2026-10-03. Automatic chase drafting remains outside this change.

Validation: full PostgreSQL-backed backend suite, build and vet; frontend date/DST tests, typecheck, lint and both builds; real web and embedded screens in light/dark at 1180 and 768 pixels. Tested Chase membership and order, optional time editing, source disclosure and persisted manual clears. Seven live gpt-6-luna drafting examples pass. The live probe exposed an offset error across the London clock change; the planner now returns local time and Go applies the timezone.

Review: date precision is a schema type; query sorting/filtering uses one deadline conversion. Existing record-write transactions enforce completion, source precedence and draft rules. Manual dates use the existing explicit-override convention used by the composer; the automatic drafting agent retains only its read-only tools. No dependencies added. Old dates retain their value and audit history; agent-authored legacy dates are Suggested because their original rationale was not recorded.
