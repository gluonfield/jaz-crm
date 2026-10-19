# CRM Keyboard Shortcuts Request

Source: Augustinas, 19 October 2026; compare Tasks implementation `9bede9d1eb8413597ae9ac6d2660306345b8f0de`.

- [x] Read CRM rules and Tasks transcript, bindings, menu and help design.
- [x] Canonical record identity and links; Copy ID and Copy link in page and row menus.
- [x] Central action registry, command menu, help, typing protection and handler inventory.
- [x] Complete mapping and separately list unsupported Linear actions.
- [x] Synthetic-only hidden browser checks: each record kind, menu and shortcut, typing, navigation, copy failures, narrow and light/dark screenshots.
- [x] Negative controls, full Go/frontend verification and thermo-nuclear review; resolve findings.
- [x] Prepare the reviewed, verified branch for approval; commit and evidence accompany the report.
- [ ] After approval: integrate main, rerun checks, push main and verify deployment.

Shared UI proposal: put the platform key matcher, scoped dispatcher and help-row layout in `@jaz/ui`; keep app action definitions and route selection in each app. This change keeps the shared package pin unchanged pending that cross-repository decision.
