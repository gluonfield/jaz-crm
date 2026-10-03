# Gmail sending reliability

- [x] Compare the reported CRM and Gmail UI messages using their original MIME source.
- [x] Diagnose missing sender name: sendAs.displayName is empty when Gmail uses the Google account name; the live UserInfo endpoint returns it.
- [x] Check domain authentication: SPF authorizes Google and google._domainkey publishes a key; _dmarc is absent. Sent copies cannot verify recipient-side SPF/DKIM/DMARC results or inbox placement.
- [x] Match sender identity, signature separator and configured Reply-To; verify new email and replies.
- [x] Keep uncertain sends claimed, preserve quota errors, and persist acknowledged outcomes after request cancellation.
- [x] Claim the exact reviewed draft and save its recipients atomically; concurrent requests cannot reclaim an active send or discard an intervening edit.
- [x] Run full Go build/vet/PostgreSQL suite, frontend test/typecheck/lint/build, focused race checks and strict maintainability review. Record writes reuse the existing optimistic check with list-valued expectations; the public scalar expect API remains compatible.
- [ ] Commit/push and verify production deployment.
- [ ] Establish expected daily volume and whether traffic is individual conversations or bulk outreach; apply the appropriate Google requirements.

No inbox-placement guarantee is possible. CRM sends through the connected Gmail account. Google account limits apply across Gmail UI, API and other clients. Two pre-existing messages from Augustinas received in the connected CAS teammate mailbox passed SPF and aligned DKIM and have INBOX labels; these are domain-authentication evidence, not a test of this patch. DMARC remains absent. No production email or DNS mutation is part of automated tests.

Google's sending API can acknowledge a message before downstream failure. Sent means Gmail accepted a message ID, not delivered or placed in the inbox. Current CRM has individually confirmed sends; bulk scheduling, mailbox pacing and automatic bounce suppression are separate capabilities. Daily volume and campaign requirements await the user's answer.

Regression controls: /tmp/crm-email-reliability-47sm3q8z. Replacing the send flow with the pre-change version reproduces cancellation, concurrent-edit and quota-error failures; the forced overlapping-send control sends two emails. Removing the profile fallback loses the sender name. Race-enabled integration checks pass.

References: https://support.google.com/mail/answer/81126 ; https://developers.google.com/workspace/gmail/api/guides/handle-errors ; https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.settings.sendAs
