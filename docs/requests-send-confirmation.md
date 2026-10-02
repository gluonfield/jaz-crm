# Send confirmation and embedded access

- [x] Diagnose the reported “only a person sends or approves a draft” error: the embedded app called Send with its host's agent credential.
- [x] Implement and locally verify Send/Cancel inside Jaz, with no browser redirect. The previously shipped browser handoff was rejected by Augustinas on 2026-10-02.
- [ ] Resolve embedded sending with human authentication. The server's bearer restriction remains in force; the dialog shows the limitation and disables confirmation instead of making a rejected request.
- [ ] Deploy the redirect removal and verify the production MCP resource loaded by Jaz.
- [x] Show sender, all recipients, message and signature with Send/Cancel; keep reviewed values stable and prevent repeated clicks.
- [x] Verify browser confirmation and embedded cancellation using controlled responses. The sandboxed MCP app opens its own dialog, issues no open-link or send request, and explains its sending limitation. Browser Send remains functional, confirms once and preserves the reviewed workspace.
- [x] Run full Go build/vet/tests and frontend typecheck/lint/web/MCP builds. Both themes and narrow/desktop layouts pass browser checks; embedded checks pass at both widths. Evidence: `/tmp/crm-inline-confirmation-20261002/results.json`.

Augustinas reported the production redirect again. Shipping its removal independently of the unresolved sending authentication; no permission boundary is relaxed and no real messages were sent during verification. Full verification passes on the final revision. The send-authentication request remains open.
