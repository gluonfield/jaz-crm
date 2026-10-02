# Draft lifecycle

- [x] Clear a draft after a confirmed successful email send, preserving sent history and failed-send recovery.
- [x] Let a later incoming message generate a fresh draft, with a specific explanation when drafting is skipped.
- [x] Repair existing sent drafts and retry the reported incoming reply.
- [x] Verify browser and embedded CRM behavior, run the full checks and code-quality review, commit and push, and verify production activation.

Verification: Go build, vet and full tests passed; frontend typecheck, lint and both builds passed. Browser checks exercised failed-send retention, successful-send clearing and fresh editable drafts in the website and embedded app, including downloaded production bundles. Code-quality review kept draft removal in the atomic record transition and editor reset tied to persisted status.

Commit `a72282b` reached production Server and Worker successfully; `/healthz` returns OK. Migration 29 removed the legacy sent draft while preserving its history. Requeued the reported empty skipped outcome after the new Worker became active; drafting completed and produced a new open email draft for the latest incoming request. No verification email was sent.

Sync timing: the current polling interval is five minutes. A shorter interval is feasible within Gmail's quota; this change preserves the current schedule.
