# Simple CRM messages

- [x] Replace nested public parts with note text, attributed messages and speaker turns.
- [x] Import messages with channel, sender, recipients and original date; preserve date-only precision.
- [x] Keep provenance outside readable content and render notes once with actual authorship.
- [x] Migrate legacy notes and the imported LinkedIn previews without losing content, links or IDs.
- [x] Exclude notes from contact statistics.
- [ ] Verify real PostgreSQL/MCP behavior, migration and the rendered application; perform strict code review.
- [ ] Commit, push and verify local and Railway deployments.

Full Go tests, vet/build and frontend typecheck/lint/web/MCP builds pass. Real PostgreSQL/MCP coverage checks message metadata, original dates, attributed transcripts, reimport replacement and note contact statistics. The old-schema migration test retains IDs, record links and original source text across a second startup. A compiling negative control restoring notes to contact statistics fails the real MCP regression. Strict review removed duplicate transport DTOs and a forwarding alias, corrected agent attribution and limited email cleanup to email bodies. No new dependency.

Production's five manual entries and their seven parts/five record links are backed up privately under `~/.jaz/backups/crm-simple-messages-2026-10-01/`. Deployment and rendered verification remain pending.
