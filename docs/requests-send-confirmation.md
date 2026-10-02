# Send confirmation and embedded access

- [x] Diagnose the reported “only a person sends or approves a draft” error: the embedded app called Send with its host's agent credential.
- [x] Route embedded Send to a signed-in browser confirmation in the same workspace; retain the server's agent restriction.
- [x] Show sender, all recipients, message and signature with Send/Cancel; keep reviewed values stable and prevent repeated clicks.
- [x] Verify browser and embedded entry paths, cancellation, success, failures, changed drafts and authenticated sending with a fake Gmail boundary.
- [x] Run full Go build/vet/tests and frontend typecheck/lint/web/MCP builds. Both themes and narrow/desktop layouts pass browser checks.
- [ ] Commit, push and verify deployment. No real messages are sent during verification.
