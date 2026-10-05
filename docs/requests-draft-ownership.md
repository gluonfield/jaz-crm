# Draft ownership requests

- [x] Confirm who created the Ujjwal outreach drafts shown under Augustinas's account.
- [ ] Fix the new-email sender mismatch first: preserve the assigned sender across viewers, permissions and concurrent edits.
- [ ] Add a quick filter for the viewer's own follow-ups after the sender fix is shipped.

## Evidence

On 2026-10-05, record `nnJjzLosWE7YxVbqUb85tN` (Russell Haigh) was created by an agent authenticated as Ujjwal Pandey at 15:20:45 UTC. Its owner is correctly `ujjwal@cambridgeadvancedsystems.com`; the live sender preview returned Augustinas's mailbox and signature when Augustinas viewed it. There are 146 open Email drafts owned by Ujjwal, created today without linked conversations. History of every draft attributes its original text to Ujjwal's account (66 agent-source, 80 user-source writes). The new-email sender path ignores `owner` and defaults to the viewer's mailbox.

No draft was sent during investigation or verification.

## Sender verification

The assigned member's connected mailbox supplies From and signature for new emails. Missing, inactive or restricted mailboxes block the draft instead of substituting the viewer. Owner changes are included in the atomic send claim. Replies and imported Gmail drafts retain their existing mailbox selection.

Full Go build, vet and tests, and frontend check pass. Real storage and both HTTP/MCP release transports verify two different viewers, sender/signature, restricted and disconnected mailboxes, mailbox aliases and concurrent reassignment. Replacing the sender path with the previous implementation reproduces the wrong From; removing the owner from the claim permits the concurrent send and fails the regression test. Maintainability review completed without outstanding findings.
