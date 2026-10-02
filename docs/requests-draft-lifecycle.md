# Draft lifecycle

- [x] Clear a draft after a confirmed successful email send, preserving sent history and failed-send recovery.
- [x] Let a later incoming message generate a fresh draft, with a specific explanation when drafting is skipped.
- [ ] Repair existing sent drafts and retry the reported incoming reply.
- [ ] Verify browser and embedded CRM behavior, run the full checks and code-quality review, commit and push, and verify production activation.

Verification: Go build, vet and full tests passed; frontend typecheck, lint and both builds passed. Browser checks exercised failed-send retention, successful-send clearing and fresh editable drafts in the website and embedded app. Code-quality review kept draft removal in the atomic record transition and editor reset tied to persisted status. Production repair and activation remain pending.
