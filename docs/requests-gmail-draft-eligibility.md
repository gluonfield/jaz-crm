# Gmail draft eligibility

- [x] Explain the reported empty, contactless follow-up from its actual provider snapshot and creation path.
- [x] Keep Gmail drafts without authored message text or an external recipient out of Follow-ups, including signature-only drafts.
- [x] Repair the reported entry without altering its Gmail original or losing CRM history.
- [x] Verify unfinished-to-ready transitions, existing Gmail draft lifecycle, full checks and strict code-quality review.
- [ ] Commit, push and verify production.

The reported record was created at 16:10 UTC by Gmail draft import. Its subject is empty, its body contains only Ujjwal's Gmail signature, and its sole recipient is Augustinas's workspace address. The importer only checked for any recipient and imposed no content requirement.

Imported drafts retain their provider snapshots while incomplete. A ready draft creates review work; clearing its message or leaving only internal recipients dismisses that review, and completing it reopens the same record. Sender validation uses the same authored-text check. Gmail HTML signature and quoted-history blocks are excluded only for eligibility; original message content remains intact.

Verification: full Go 1.26 tests/build/vet and frontend tests/typecheck/lint/both builds pass. PostgreSQL-backed Gmail API integration cases cover empty, unaddressed, plain/HTML signature-only, quoted-only, internal-only, new external contacts, mixed internal/external recipients, and authored text before/after a signature. The existing attachment/Bcc/send/edit/delete lifecycle suite passes. Original-importer and original-send negative controls reproduce the failures. Strict review found no material issues; policy stays in the importer, content detection stays in the Google adapter, and no dependencies or UI changes were added.

Production repair set the reported record to Dismissed through the normal CRM API. The draft text, recipients and history remain; Gmail still reports the original draft ID and unchanged message revision. No mail was sent.
