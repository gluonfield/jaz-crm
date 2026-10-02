# Drafting context

- [x] Keep drafting as one structured LLM call, with no model tool loop.
- [x] Supply the full stored conversation rather than its last 24,000 bytes.
- [x] Supply the workspace's Company page and its nested pages from current records.
- [x] Supply linked people, their companies, related deals and open follow-ups with their current data.
- [x] Identify the sender explicitly when the conversation has a known owner.
- [x] Verify the actual LLM request through the production storage and client paths, review, run the full backend suite, commit and push.

Company knowledge is selected from the workspace's pages named Company and their descendants. Conversation context uses the complete stored thread; separate past conversations remain represented by the person's existing relationship summary. Records and documents are context, and may include internal information that should not be copied into external replies.

Verification: Go 1.26 build, vet and the full backend suite passed against an isolated PostgreSQL database. The real Agent → LLM HTTP request test covers messages and documents longer than 24 KB, original quoted content, 103 Company documents across multiple query pages, current contact/company/deal values, sender identity, workspace isolation, and refreshed page content on the next conversation change. Three temporary Go overlays each made the regression test fail as expected: restoring display cleanup, cutting message text, and reading only the first document page. Test databases were removed by their cleanups.

Review: context collection stays in followups, original/display text selection stays in interactions, and the structured model call stays unchanged. Source-read failures release the conversation claim for retry. No schema migration, dependency or model tool loop is introduced. Oversized provider requests fail instead of silently dropping input; existing saved drafts are regenerated only when their conversation changes.
