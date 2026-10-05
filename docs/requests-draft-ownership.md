# Draft ownership requests

- [x] Confirm who created the Ujjwal outreach drafts shown under Augustinas's account.
- [x] Fix the new-email sender mismatch first: preserve the assigned sender across viewers, permissions and concurrent edits.
- [x] Add a quick filter for the viewer's own follow-ups after the sender fix is shipped.

## Evidence

On 2026-10-05, record `nnJjzLosWE7YxVbqUb85tN` (Russell Haigh) was created by an agent authenticated as Ujjwal Pandey at 15:20:45 UTC. Its owner is correctly `ujjwal@cambridgeadvancedsystems.com`; the live sender preview returned Augustinas's mailbox and signature when Augustinas viewed it. There are 146 open Email drafts owned by Ujjwal, created today without linked conversations. History of every draft attributes its original text to Ujjwal's account (66 agent-source, 80 user-source writes). The new-email sender path ignores `owner` and defaults to the viewer's mailbox.

No draft was sent during investigation or verification.

## Sender verification

The assigned member's connected mailbox supplies From and signature for new emails. Missing, inactive or restricted mailboxes block the draft instead of substituting the viewer. Owner changes are included in the atomic send claim. Replies and imported Gmail drafts retain their existing mailbox selection.

Full Go build, vet and tests, and frontend check pass. Real storage and both HTTP/MCP release transports verify two different viewers, sender/signature, restricted and disconnected mailboxes, mailbox aliases and concurrent reassignment. Replacing the sender path with the previous implementation reproduces the wrong From; removing the owner from the claim permits the concurrent send and fails the regression test. Maintainability review completed without outstanding findings.

`fafcf0d` is pushed to main and deployed on Server and Worker; GitHub verification and publishing succeeded. Authenticated live sender preview for Russell now returns Ujjwal's mailbox and Gmail signature while Augustinas is viewing it. The existing records needed no reassignment or body edits.

## Own-work filter

Follow-ups now has an owner picker beside search, in both queue and table. Everyone clears only owner conditions; Assigned to me uses the authenticated workspace member; teammate names select their work. This applies across channels. The existing route filters preserve the selection on reload, alongside status, search and sort; there is no extra saved preference or server state.

Actual built app and real HTTP/storage verification on a disposable database: Everyone shows three open actions; Assigned to me shows the viewer's Email and LinkedIn actions and excludes the teammate and completed action. Queue/table selection and reload pass, and clearing owner preserves status, text and sort. Light/dark rendering and 390px frame geometry verify the new control and search stay within the pane. The existing sidebar limits the rest of the screen at this width. Maintainability review has no outstanding findings; all frontend checks pass. Production readback follows the push.
