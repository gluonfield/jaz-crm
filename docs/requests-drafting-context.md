# Drafting context

- [x] Keep each person's name, fields and relationship summary together, removing duplicate input contexts.
- [x] Select Company knowledge through a persisted workspace page reference, preserve existing unambiguous Company roots, and include readable descendant paths.
- [x] Verify multiple conversation contacts, exclusion of unrelated employees, root renames, pagination and workspace isolation through the real request path.
- [x] Review and run all required checks before committing and pushing the correction.

- [x] Keep drafting as one structured LLM call, with no model tool loop.
- [x] Supply the full stored conversation rather than its last 24,000 bytes.
- [x] Supply the workspace's Company page and its nested pages from current records.
- [x] Supply linked people, their companies, related deals and open follow-ups with their current data.
- [x] Identify the sender explicitly when the conversation has a known owner.
- [x] Verify the actual LLM request through the production storage and client paths, review, run the full backend suite, commit and push.

Company knowledge starts at the workspace's persisted `company_page_id` and includes every descendant's full content and root-relative path, such as `Company / Strategy`. Admins select it with `update_workspace`; omission preserves it and an empty string clears it. The page must belong to the workspace. Renaming preserves the selection; deleting the root clears it. Migration 0027 selects an existing unambiguous top-level Company page once. Ambiguous roots need explicit selection, and new workspaces have no implicit root.

Each person appears once in `records`, with their name and `values.context`; participants carry the corresponding `person_id` when known. Selection starts from conversation-linked records and expands to their companies, deals and open follow-ups, without pulling in every employee. The output `contexts` array remains the list of summary updates. Conversation context uses the complete stored thread; separate past conversations remain represented by the person's existing relationship summary. Records and documents may contain internal information that should not be copied into external replies.

Verification: Go 1.26 build, vet and the full backend suite passed against an isolated PostgreSQL database. The real Agent → LLM HTTP request test covers messages and documents longer than 24 KB, original quoted content, 103 Company documents across multiple query pages, current contact/company/deal values, sender identity, workspace isolation, and refreshed page content on the next conversation change. Three temporary Go overlays each made the regression test fail as expected: restoring display cleanup, cutting message text, and reading only the first document page. Test databases were removed by their cleanups.

Correction verification: Go 1.26 build, vet and the full backend suite passed. The request test now exercises sync links for two contacts at one company, excludes an unrelated colleague and same-title page, verifies complete document paths after renaming the root, and verifies clearing prevents title-based fallback. MCP tests cover persistence, omission, clearing, deletion, admin permissions, workspace isolation and atomic rejection of invalid roots. Migration tests cover absent, unique and ambiguous roots, including nested same-title pages.

Review: context collection stays in followups, original/display text selection stays in interactions, and root selection belongs to workspace settings. Selection validation and workspace updates happen in one SQL write. The traversal uses one path map for both paths and deduplication. Source-read failures release the conversation claim for retry. No dependency or model tool loop is introduced. Oversized provider requests fail instead of silently dropping input; existing saved drafts are regenerated only when their conversation changes.
