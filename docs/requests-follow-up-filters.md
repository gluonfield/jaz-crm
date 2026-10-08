# Follow-up Custom Filters

Source: Augustinas, 2026-10-08; Follow-ups screenshot and request for LinkedIn-only and Email-only filters.

- [x] Expose the existing condition editor in the Follow-ups queue.
- [x] Allow Channel is LinkedIn or Email, combined with owner and other conditions.
- [x] Save, select and edit named views through the existing workspace presets.
- [x] Preserve Assigned to me as the default and client-local active filters.
- [x] Verify real queue/table, nested pickers, save/reload, light/dark and narrow layouts; run full checks and strict review.
- [x] Commit, push and verify deployment.
- [x] Run the requested thermo-nuclear review and reproduce delayed-write behavior.
- [x] Preserve newer view/editor state when an earlier save or deletion completes.
- [x] Verify the review fix with delayed real responses and full checks.
- [ ] Commit, push and verify the review-fix deployment.

Reuse the existing filter editor, URL state and saved-filter API. No new dependencies or persistence layer.

Full frontend checks, Go tests/build/vet and strict review pass. A disposable PostgreSQL-backed browser stack verifies LinkedIn-only and Email-only rows, combined owner/status conditions, nested field/value pickers, Save as, saved-view switching, Save changes, reload, default assignment and the table variant. Light/dark screenshots and a 572px viewport with a 340px queue verify wrapping and bounded popovers without page overflow. Temporary fixtures live outside the repository.

`575987695d8a4f1b629ebf8a3370fe96fa21c0b8` is pushed and live on Server and Worker, both SUCCESS. CI verification and health pass. The production queue exposes Filter, Channel choices and Save as; closing the editor preserves the current filters and Assigned to me. No production records or presets were changed by verification.

The requested second review found a P2 in the existing shared editor: after closing a pending save and selecting another view, the late response reapplied the old view. The component now detaches mutation callbacks when its view or editor context changes, using React Query reset. The backend write and cache refresh still complete. Real PostgreSQL-backed browser checks reproduce the original defect and verify delayed save/deletion responses preserve the newer selection, saved presets remain readable, newer name/condition edits survive old responses, normal Save as/Save changes work, and fresh navigation still defaults to Assigned to me. Full frontend checks pass with no lint warnings. No new flags, helpers, APIs or dependencies.
