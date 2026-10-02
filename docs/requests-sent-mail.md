# Sent email visibility

- [x] Check whether the Follow-ups email was imported and attached to records.
- [x] Correct refreshes when a connection sync completes between UI polls.
- [x] Verify the real app with an idle-to-idle sync update and an open conversation.
- [x] Run the full checks and code-quality review.
- [x] Commit, push and verify production activation.

The reported reply sent at 2026-10-02 22:21:52 UTC is stored in its original About RFQ thread, attached to Kojaddnd Nuhaeaan and both linked follow-ups. Gmail sync rebuilt its person link at 22:26:11 UTC. At that time, sending relied on the five-minute connection sync to import the reply. The UI now invalidates cached views when connection progress changes, including updated stream timestamps while the observed activity stays idle.

Verification: the deployed baseline leaves an open full thread stale after an idle-to-idle history update; the corrected website and sandboxed MCP App render the new sent message without reloading and retain the unsent draft. Ongoing busy-sync refreshes also pass. Frontend typecheck, lint and both builds pass; Go 1.26 build, vet and the full backend suite against isolated PostgreSQL pass. Review keeps cache invalidation in the shared sync hook, preserves active-pass polling and adds no send, storage or dependency changes. Evidence: `/tmp/crm-sent-sync-20261002/results.json` and check logs in the same directory.

Deployed `1ede8fb0c6c20f4d0a66894cb233d6c0a6b26355`: Server `9c393e31-9727-4d00-b62e-d75bfeea46b9` and Worker `3948054f-be8f-4146-83e3-dc3c2ccf067a` reached SUCCESS. Production health is 200; the authenticated MCP resource matches the verified local bundle. Browser checks against deployed web assets and the deployed MCP resource pass for completed-between-polls sync, ongoing busy sync and retained drafts. Evidence: `/tmp/crm-sent-sync-20261002/deployed-results.json`. GitHub backend verification passed; registry image publishing was still running at handoff. No email was sent during verification.

## One-minute polling

- [x] Change the connection polling interval from five minutes to one minute.
- [x] Preserve Gmail history and Calendar sync cursors and push-triggered wakes.
- [x] Verify workflow pacing, run full backend checks and review the change.
- [ ] Commit, push and verify production polling.

The workflow now waits one minute after a pass finishes. Its existing cursor-based fetches, activity retries and immediate catch-up passes remain in place. The workflow test checks an actual one-minute idle interval and a wake halfway through the next interval. No email or forced draft generation is needed for verification.

One-minute validation: Go 1.26 build, vet and full backend tests pass. Review changes one owning interval and adjusts the existing behavioral pacing test; no dependencies, polling machinery or cursor changes. Temporal SDK timer replay compares timer identity, allowing existing workflows to adopt the new duration after their current wait or a wake.
