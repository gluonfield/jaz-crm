# Workspace settings and drafting

- [x] Split settings into Triage, Team, Schema and Drafting subpages, preserving workspace administration and access settings.
- [x] Select only pages for company knowledge; include child pages and preserve existing configuration.
- [x] Add a persisted, enforced web-access setting, off by default.
- [x] Always include sender identity, related records, full current conversation and full stored history for its contacts.
- [x] Explain Temporal retry boundaries: sync workflow activities retry; the drafting loop currently runs inside one activity and retries from the start.
- [x] Verify persistence, authorization, prompt contents, tool gating, page navigation, desktop/narrow layouts and web/embedded surfaces.
- [x] Complete strict code-quality review and all local checks.

Publication: commit and push the verified change; verify both production services before reporting it live. Deployment evidence is recorded in the session handoff.

Verification: Go 1.26 build, vet and full PostgreSQL-backed test suite; frontend typecheck, lint and build; real MCP/PostgreSQL browser checks of both surfaces in light/dark themes at desktop and narrow widths; live synthetic OpenAI web-search draft.
