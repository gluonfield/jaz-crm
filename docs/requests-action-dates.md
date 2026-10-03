# Action dates and Chase

## Urgent incoming replies, 2026-10-03

- [x] Treat an incoming response as due at the message's local arrival time, including weekends, so urgency starts immediately.
- [x] Replace the receipt-day convention: date-only deadlines allow until the end of the day.
- [x] Verify real model plans for immediate and deferred responses, the existing timed-deadline path, full backend checks and strict review.
- [x] Commit/push and verify the active Server and Worker revisions.

Validation: eleven live model cases cover first/repeated requests, an existing date-only Today action, a reply while waiting on Them, manual dates, explicit waits, no action, unavailable pricing, overdue work and a future deliverable. Incoming actions use the local arrival minute; the old prompt fails both ordinary immediate-response cases. The unavailable-price case supplies the fact that enterprise pricing is not established, so a truthful acknowledgement is valid; the original probe's no-draft requirement was overly restrictive. Full Go 1.26 build/vet/PostgreSQL suite passes on the final source revision; frontend tests and a production date-helper check confirm that a timed arrival deadline is overdue immediately while a date-only deadline is not. Strict review keeps the correction in the existing planner, using the existing timezone conversion and urgency grouping without new branches or dependencies.

Production: `10ddd51` is SUCCESS on Server (`c18a61b5-7249-47c5-8095-778301bed1bf`) and Worker (`addf13a3-6a12-42c6-99ed-f5fb58389dcb`); health passes. New incoming response actions use receipt-time scheduling after sync and drafting. Existing manual scheduling and reviewed sending are preserved.

## Immediate replies, 2026-10-03

- [x] Diagnose why a new incoming request retained a future Suggested response date: the planner defaulted to three working days and preserved existing dates.
- [x] Make incoming messages needing an answer due on their local receipt day, including weekends; replace future Suggested response dates on subsequent requests.
- [x] Keep concrete reasons to wait, manual overrides and separate commitments; explain intentional deferral or no reply.
- [x] Verify real model plans for new/repeated messages and exceptions, full checks and strict review.
- [x] Recheck the reported follow-up before correcting its date: it was completed by the user's reply during the investigation, so no pending response remains to reschedule.
- [x] Commit/push and verify production: `7a81532` is SUCCESS on Server and Worker; health passes.

Validation: nine live gpt-6-luna plans cover first/repeated incoming requests, manual dates, an explicit wait, waiting on Them, no reply needed, missing facts, overdue responses and a later deliverable with an immediate acknowledgement. The original prompt fails both immediate-response cases. All backend build/vet/PostgreSQL tests and frontend tests/typecheck/lint/builds pass. Strict review keeps scheduling judgment in the existing planner and removes the three-working-day convention; no new branches, transport changes or dependencies.

A further live plan confirms that receiving another question while waiting on Them creates an Us response due today. A distinct outstanding request may remain its own follow-up.

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

## New email drafts, 2026-10-03

- [x] Send a draft with no previous email as a new email; retain reply threading for existing conversations.
- [x] Allow agents to propose a separate subject, including after calls/notes; separate legacy leading Subject lines from existing draft bodies.
- [x] Show editable subject and recipients in the composer and subject in confirmation; reject stale subject confirmations and protect approval on edits.
- [x] Verify both transports, new-email and reply delivery, agent persistence, migration, real screens, full checks and strict review.
- [x] Commit/push and verify production.

Verification: full backend build/vet/PostgreSQL suite and frontend checks pass. Real website/embedded app checks pass in light/dark and narrow layouts: subject/To/Cc/body editing, reload persistence, confirmation, stale-subject rejection, and sending through the real API with Gmail isolated at its HTTP boundary. A live gpt-6-luna probe proposes a distinct subject after a call and inherits the subject for an existing reply. Migration tests preserve explicit subjects, other channels, in-flight sends, original text/source and draft timestamps.

Review: preview and sending now share one message composer with optional reply metadata. Subject uses the existing record schema, approval guard and atomic sent-draft cleanup. Editor state and saves move together into one composer component. No dependencies added. Legacy subject extraction preserves the original writing time so a migration cannot make stale text look freshly reviewed. Confirmation retains the reviewed message and save callback; background changes are not silently saved over before sending. Gmail thread IDs are used only with matching subjects, following https://developers.google.com/workspace/gmail/api/guides/threads.

Production: `fe5ffa9` is pushed and SUCCESS on Server and Worker. Health, live save/send subject schemas and exact embedded resource match pass. The reported Dan draft now has a separate subject and its original body; the prior combined text remains in history. Every existing Follow-ups schema has the subject attribute.
