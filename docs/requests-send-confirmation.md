# Send confirmation and embedded access

- [x] Diagnose the reported “only a person sends or approves a draft” error: the embedded app called Send with its host's agent credential.
- [x] Implement and locally verify Send/Cancel inside Jaz, with no browser redirect. The previously shipped browser handoff was rejected by Augustinas on 2026-10-02.
- [ ] Resolve embedded sending with human authentication. The server's bearer restriction remains in force; the dialog shows the limitation and disables confirmation instead of making a rejected request.
- [x] Deploy the redirect removal. Server `82f793cd-6f81-4dcf-b566-ddbcce9ef700` and Worker `392d8dfb-4a6e-436d-8653-ae7809523e35` reached SUCCESS at `aacefa1`; production web-asset checks pass. The built MCP app passes sandboxed UI checks; the actual open Jaz frame cannot be inspected through the available tools. Reopening CRM reloads its resource.
- [x] Show sender, all recipients, message and signature with Send/Cancel; keep reviewed values stable and prevent repeated clicks.
- [x] Verify browser confirmation and embedded cancellation using controlled responses. The sandboxed MCP app opens its own dialog, issues no open-link or send request, and explains its sending limitation. Browser Send remains functional, confirms once and preserves the reviewed workspace.
- [x] Run full Go build/vet/tests and frontend typecheck/lint/web/MCP builds. Both themes and narrow/desktop layouts pass browser checks; embedded checks pass at both widths. Evidence: `/tmp/crm-inline-confirmation-20261002/results.json`.

Augustinas reported the production redirect again. Shipping its removal independently of the unresolved sending authentication; no permission boundary is relaxed and no real messages were sent during verification. Full verification passes on the final revision. The send-authentication request remains open.
