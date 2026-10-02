# Follow-up context and composer

- [x] Make the context visible and the expanded follow-up easier to understand.
- [x] Show progress while the background worker reviews messages and drafts replies.
- [x] Let people write a reply when no generated draft exists, including during background generation.
- [x] Preserve typing through background refreshes and generation; saved human drafts retain priority.
- [x] Review and verify the UI and persistence behavior.

Use the existing connection sync stage for queue-level progress, rather than inventing a per-message status. Reuse the linked conversation to determine the reply channel and the existing Gmail sender preparation for recipients. Preserve human approval and the surrounding queue layout.

Verification: Go build, vet and full suite against PostgreSQL; frontend typecheck, lint and both builds; browser checks through the production HTTP handlers for empty replies, concurrent generation, rapid saves and opening the original message. Delivery and production deployment are recorded in the session memory.

## Inline conversation revision

- [x] Show the latest full message as a chat bubble directly above the composer.
- [x] Expand the complete thread above that message, oldest to newest, with a Show thread button.
- [x] Remove Context & reply; keep opening a follow-up through its row.
- [x] Make Person context a muted uppercase section label.
- [x] Have Claude Opus 5.5 at xhigh refine the implemented design as a world-class product manager.
- [x] Make person context a clearly bounded section that can be collapsed.
- [x] Align the recipient's incoming message on the right, with sent messages on the left.
- [x] Verify both themes, narrow layout, thread order and editable replies; complete design and code reviews.

Verification: frontend typecheck, lint and both builds; full Go suite against isolated PostgreSQL; real built-app browser checks using captured production responses for full messages, chronological thread expansion, empty composition, context collapse and a 380px content width. Claude Opus 5.5 at xhigh completed both design passes. Commit, push and production deployment are recorded in the session memory.
