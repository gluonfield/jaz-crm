# Sent email visibility

- [x] Check whether the Follow-ups email was imported and attached to records.
- [x] Correct refreshes when a connection sync completes between UI polls.
- [x] Verify the real app with an idle-to-idle sync update and an open conversation.
- [x] Run the full checks and code-quality review.
- [ ] Commit, push and verify production activation.

The reported reply sent at 2026-10-02 22:21:52 UTC is stored in its original About RFQ thread, attached to Kojaddnd Nuhaeaan and both linked follow-ups. Gmail sync rebuilt its person link at 22:26:11 UTC. Sending currently relies on the five-minute connection sync to import the reply. The UI now invalidates cached views when connection progress changes, including updated stream timestamps while the observed activity stays idle.

Verification: the deployed baseline leaves an open full thread stale after an idle-to-idle history update; the corrected website and sandboxed MCP App render the new sent message without reloading and retain the unsent draft. Ongoing busy-sync refreshes also pass. Frontend typecheck, lint and both builds pass; Go 1.26 build, vet and the full backend suite against isolated PostgreSQL pass. Review keeps cache invalidation in the shared sync hook, preserves active-pass polling and adds no send, storage or dependency changes. Evidence: `/tmp/crm-sent-sync-20261002/results.json` and check logs in the same directory.
