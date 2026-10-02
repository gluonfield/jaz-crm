# Draft generation state

- [x] Investigate why statukaipapi's RFQ/CNC message has no generated reply. The agent created a follow-up, while its prompt permits an empty draft when the answer needs unavailable facts; its exact response was not retained.
- [ ] Force a fresh production drafting pass for this conversation and verify its outcome.
- [x] Show Drafting, Completed, Failed with reason, and Skipped with reason beside the reply box.
- [x] Persist actual attempt outcomes, distinguish an intentional empty reply from an error, and recover interrupted attempts.
- [x] Refresh status and completed drafts while preserving human edits.
- [x] Run database/service/HTTP and browser verification, including both themes and narrow layouts.
- [ ] Commit, push and verify production activation.

Use the existing conversation and worker. Sending remains the existing explicitly approved action. This work is isolated from concurrent drafting-context changes in the primary checkout.
