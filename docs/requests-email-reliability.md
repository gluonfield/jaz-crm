# Gmail sending reliability

- [x] Compare the reported CRM and Gmail UI messages using their original MIME source.
- [x] Diagnose missing sender name: sendAs.displayName is empty when Gmail uses the Google account name; the live UserInfo endpoint returns it.
- [x] Check domain authentication: SPF authorizes Google and google._domainkey publishes a key; _dmarc is absent. Sent copies cannot verify recipient-side SPF/DKIM/DMARC results or inbox placement.
- [x] Match sender identity, signature separator and configured Reply-To; verify new email and replies.
- [x] Keep uncertain sends claimed, preserve quota errors, and persist acknowledged outcomes after request cancellation.
- [x] Claim the exact reviewed draft and save its recipients atomically; concurrent requests cannot reclaim an active send or discard an intervening edit.
- [x] Run full Go build/vet/PostgreSQL suite, frontend test/typecheck/lint/build, focused race checks and strict maintainability review. Record writes reuse the existing optimistic check with list-valued expectations; the public scalar expect API remains compatible.
- [x] Commit/push and verify production deployment: c748d18 is SUCCESS on Server (38bf8f20) and Worker (999e3a86); /healthz returns ok.
- [ ] Establish expected daily volume and whether traffic is individual conversations or bulk outreach; apply the appropriate Google requirements.

No inbox-placement guarantee is possible. CRM sends through the connected Gmail account. Google account limits apply across Gmail UI, API and other clients. Two pre-existing messages from Augustinas received in the connected CAS teammate mailbox passed SPF and aligned DKIM and have INBOX labels; these are domain-authentication evidence, not a test of this patch. DMARC remains absent. Regular automated tests use provider fixtures. The explicitly requested live draft verification sent only controlled self-addressed mail, with test messages trashed afterward. No customer draft or DNS record was changed by these probes.

Google's sending API can acknowledge a message before downstream failure. Sent means Gmail accepted a message ID, not delivered or placed in the inbox. Current CRM has individually confirmed sends; bulk scheduling, mailbox pacing and automatic bounce suppression are separate capabilities. Daily volume and campaign requirements await the user's answer.

Regression controls: /tmp/crm-email-reliability-47sm3q8z. Replacing the send flow with the pre-change version reproduces cancellation, concurrent-edit and quota-error failures; the forced overlapping-send control sends two emails. Removing the profile fallback loses the sender name. Race-enabled integration checks pass.

References: https://support.google.com/mail/answer/81126 ; https://developers.google.com/workspace/gmail/api/guides/handle-errors ; https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.settings.sendAs

## Follow-up reports

- [x] Diagnose the Kojaddnd/Statukai rows: they are completed historical replies, never simultaneously open; preserve both rather than merging or deleting history.
- [ ] Deploy explicit Done/Dismissed history presentation and enforced reuse of open conversation reply actions; preserve separate deliverables and commit each model plan atomically.
- [x] Preserve Gmail draft state independently from authorship; exclude unsent drafts from sent activity and waiting-on-them reasoning.
- [x] Import Gmail drafts into the review/send workflow with subject, body, recipients, originating mailbox and stable provider draft identity.
- [x] Reconcile edits, provider sending and deletion; ensure CRM sending consumes the existing Gmail draft exactly once.
- [ ] Repair David Clowsley interaction d2decb96-16f9-48d1-a7ab-9c10f6fc2561 and its incorrect waiting follow-up; audit other imported drafts.
- [x] Audit current CAS Gmail drafts: August has none; Ujjwal has four, all imported as messages. Correct David, Lisa and Chloe's contact summaries to say an unsent draft was prepared, preserving the dated event and unrelated context.
- [ ] Migrate existing message/follow-up entries to drafts while preserving identity, content, recipients, mailbox and history; user explicitly rejected deleting them.
- [ ] Verify the lifecycle with a controlled real Gmail draft, plus automated regression checks, strict review and production rollout.

Live lifecycle checks passed through the production Go services with real Gmail and isolated PostgreSQL: import, same-ID provider edits, stale edit refusal, CRM edit, one confirmed CRM send, external Gmail send, and deletion. Actual sent MIME retained the display name, HTML/text alternatives, Bcc and attachment. UI checks cover overlapping autosaves, confirmation revisions, sending-state polling, newer local input and the real interaction-to-follow-up route. Evidence: /tmp/crm-gmail-drafts.c39pdI and /tmp/crm-followup-history.iwjmjwl1.

Final build, vet, full PostgreSQL-backed Go suite, lifecycle race suite and full frontend check pass. Negative controls reproduce draft-as-sent ingestion, duplicate replies, partial plan saves, stale sends, lost new composer text during send settlement, MIME body duplication and lost historical content. Strict review keeps provider synchronization in followups, MIME handling in google, durable identity/locking in storage and uses the existing composer across the queue and record routes.
