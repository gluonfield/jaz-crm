# Follow-up Custom Filters

Source: Augustinas, 2026-10-08; Follow-ups screenshot and request for LinkedIn-only and Email-only filters.

- [x] Expose the existing condition editor in the Follow-ups queue.
- [x] Allow Channel is LinkedIn or Email, combined with owner and other conditions.
- [x] Save, select and edit named views through the existing workspace presets.
- [x] Preserve Assigned to me as the default and client-local active filters.
- [x] Verify real queue/table, nested pickers, save/reload, light/dark and narrow layouts; run full checks and strict review.
- [ ] Commit, push and verify deployment.

Reuse the existing filter editor, URL state and saved-filter API. No new dependencies or persistence layer.

Full frontend checks, Go tests/build/vet and strict review pass. A disposable PostgreSQL-backed browser stack verifies LinkedIn-only and Email-only rows, combined owner/status conditions, nested field/value pickers, Save as, saved-view switching, Save changes, reload, default assignment and the table variant. Light/dark screenshots and a 572px viewport with a 340px queue verify wrapping and bounded popovers without page overflow. Temporary fixtures live outside the repository.
