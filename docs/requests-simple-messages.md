# Simple CRM messages

- [x] Replace nested public parts with note text, attributed messages and speaker turns.
- [x] Import messages with channel, sender, recipients and original date; preserve date-only precision.
- [x] Keep provenance outside readable content and render notes once with actual authorship.
- [x] Migrate legacy notes and the imported LinkedIn previews without losing content, links or IDs.
- [x] Exclude notes from contact statistics.
- [x] Verify real PostgreSQL/MCP behavior, migration and the rendered application; perform strict code review.
- [x] Commit, push and verify local and Railway deployments.

Full Go tests, vet/build and frontend typecheck/lint/web/MCP builds pass. Real PostgreSQL/MCP coverage checks message metadata, original dates, attributed transcripts, reimport replacement and note contact statistics. The old-schema migration test retains IDs, record links and original source text across a second startup. A compiling negative control restoring notes to contact statistics fails the real MCP regression. Strict review removed duplicate transport DTOs and a forwarding alias, corrected agent attribution and limited email cleanup to email bodies. No new dependency.

`7452d1e` is pushed and active on Railway server/worker and local :7500. GitHub verification and both published images succeeded; health checks return 200 and local schema version is 18. Live PostgreSQL/MCP reads confirm two LinkedIn messages with original dates and three single-body notes, all IDs/content and five record links preserved. Before/after row backups and the verifier remain private under `~/.jaz/backups/crm-simple-messages-2026-10-01/`.

Live browser acceptance confirms corrected contact totals and original-day timeline placement, message sender/recipient/date/excerpt, plain note author/body, source disclosure and opening/closing dialogs. Source details wrap without horizontal overflow at the actual 784px browser width. Dark dialogs and a light direct-note page were inspected.

| Before | After |
| --- | --- |
| Notes rendered as an unattributed message thread | One body with author/date |
| LinkedIn previews imported as note plus transcript | Attributed messages on their original days |
| Import audit mixed into the readable body | Collapsed Source details |
| Research notes advanced Last contact | Communication alone counts as contact |
