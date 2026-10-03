# Conversation queue

- [x] Reproduce the five rows in the screenshot: separate completed replies linked to one email conversation, each repeating that conversation's latest message.
- [x] Show one row per conversation in the Follow-ups queue; retain separate threads, standalone actions, raw records and message history.
- [x] Prioritize an open action over past replies and let every linked action be reviewed and selected within the conversation.
- [x] Search historical action text/recipients without hiding the current action. Apply filters and pagination with conversation counts.
- [x] Verify real database/API behavior, full Go and frontend suites/builds, and strict review. Removing grouping reproduces the five-row bug in the regression test.
- [ ] Visual and action-switch browser acceptance: blocked by side-browser CDP timeouts despite connected status.
- [x] Commit and push `9206e09`; Server `1764df36` and Worker `c993c076` are SUCCESS on that revision. Health and the production-data query pass; visual screenshot acceptance remains blocked as above.

The queue is a projection. No production records, drafts or messages are deleted or merged. The table remains a record-level view.

Only actions with a single eligible conversation are grouped. Actions spanning multiple conversations remain separate. The action selector preserves edits during delayed or failed saves; grouped pages use conversation identity to avoid duplicate rows when the representative changes between pages.

Read-only production-data check: the exact generated grouped SQL with query `statukai` returns one row, the current Open/Them booking action, for conversation `e01c5237-f67b-4033-a91f-21589b0708c8`. The same nine follow-up records remain stored.

## Strict review

- [x] Preserve the selected action and its latest record while history loads or a refresh changes the queue representative. Real React probes reproduce the original lost edit and stale saved-body fallback, then pass with the fix.
- [x] Preserve grouping and conversation scope through MCP resource links, the decoder, inline refetch and the full route. Raw history opens as an unfiltered table; scope is visible and clearable. API replay and normal frontend tests cover the contract.
- [x] Make saved-view counts use the queue's grouping and scope. A rendered component probe shows one conversation; restoring the old request reproduces five actions.
- [x] Keep record identity for keyboard navigation in the raw history table, where multiple actions share a conversation ID.
- [x] Exercise delayed saves, continued typing, failed saves and successful retries through the real composer and action menu in a temporary React DOM harness. No emails sent.
- [x] Final full Go build/vet/tests and frontend tests/typecheck/lint/web and embedded builds.
- [x] Review commit `03454d7` is live: Server `09d8fadb` and Worker `3017da50` are SUCCESS on that revision; production health returns `ok`.

Native browser visual acceptance remains blocked by Jaz CDP timeouts. React DOM checks establish state and interaction behavior, not layout or native pointer behavior.
