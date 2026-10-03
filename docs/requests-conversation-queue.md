# Conversation queue

- [x] Reproduce the five rows in the screenshot: separate completed replies linked to one email conversation, each repeating that conversation's latest message.
- [x] Show one row per conversation in the Follow-ups queue; retain separate threads, standalone actions, raw records and message history.
- [x] Prioritize an open action over past replies and let every linked action be reviewed and selected within the conversation.
- [x] Search historical action text/recipients without hiding the current action. Apply filters and pagination with conversation counts.
- [x] Verify real database/API behavior, full Go and frontend suites/builds, and strict review. Removing grouping reproduces the five-row bug in the regression test.
- [ ] Visual and action-switch browser acceptance: blocked by side-browser CDP timeouts despite connected status.
- [ ] Commit, push and verify the deployed screenshot case.

The queue is a projection. No production records, drafts or messages are deleted or merged. The table remains a record-level view.

Only actions with a single eligible conversation are grouped. Actions spanning multiple conversations remain separate. The action selector preserves edits during delayed or failed saves; grouped pages use conversation identity to avoid duplicate rows when the representative changes between pages.

Read-only production-data check: the exact generated grouped SQL with query `statukai` returns one row, the current Open/Them booking action, for conversation `e01c5237-f67b-4033-a91f-21589b0708c8`. The same nine follow-up records remain stored.
