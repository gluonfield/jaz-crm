# Follow-up context and composer

- [x] Make the context visible and the expanded follow-up easier to understand.
- [x] Show progress while the background worker reviews messages and drafts replies.
- [x] Let people write a reply when no generated draft exists, including during background generation.
- [x] Preserve typing through background refreshes and generation; saved human drafts retain priority.
- [x] Review and verify the UI and persistence behavior.

Use the existing connection sync stage for queue-level progress, rather than inventing a per-message status. Reuse the linked conversation to determine the reply channel and the existing Gmail sender preparation for recipients. Preserve human approval and the surrounding queue layout.

Verification: Go build, vet and full suite against PostgreSQL; frontend typecheck, lint and both builds; browser checks through the production HTTP handlers for empty replies, concurrent generation, rapid saves and opening the original message. Delivery and production deployment are recorded in the session memory.
