# Column reordering — 19 October 2026

- [x] Take over thread 20261019T122458-8f088112; inspect inherited changes and full transcript.
- [x] Every object table: server-persisted order, Name fixed first, headers and cells and widths stay together.
- [x] Drag with insertion indicator; accessible Move Left/Right header menu; no new dependency.
- [x] Backend persistence, MCP visibility, tenant isolation; frontend interaction tests and negative controls.
- [x] Hidden browser on synthetic local data: light/dark/narrow, before/after screenshots, drag recording.
- [x] Full backend and frontend checks; thermo-nuclear review and fixes; verify final revision.
- [x] Commit implementation and verification on this branch.
- [x] Augustinas approved release. Merge origin/main and re-run full backend/frontend checks.
- Release: fast-forward main, push and record the deployment result in the thread report.
- [x] Stop all verification processes; no test databases left behind. Worktree Postgres and preview server stopped.

## Decisions

Continue the inherited `objects.columns` design. Named presets currently hold only query and filters; they do not own table display settings. Order is shared by the workspace's table, across presets, and exposed to agents by `list_objects` / `edit_object` action `order`.

Computed columns use `$people`, `$last_contact` and `$updated_at`, distinct from attribute slugs (which start with a letter). Name is implicit and fixed first. Unlisted columns follow in their existing default order. Archived columns retain their saved position; deleted attributes disappear from the API order.

The pending push branch already watches every UPDATE on objects and invalidates list_objects, so column changes use its existing notification path. Column order is migration 0058, after main’s 0057 sidebar ordering. Push-not-poll has not landed on origin/main and must use the next free number when it lands.

## Verification

- Go 1.26: full `go test -p 2 ./...`, `go vet -p 2 ./...`, `go build -p 2 ./...` pass against worktree-local PostgreSQL 17 with synthetic data.
- Frontend `bun run check`: 25 existing tests, typecheck, lint, both builds and the new native-headless interaction test pass. `bun run test:browser` is included in that check; it uses installed Chrome/Chromium (or `CHROME`), no new dependency.
- 11/11 negative controls caught: persistence, MCP exposure, built-in ordering, fixed Name, validation, duplicate columns, deleted columns, native dragging, reload persistence, archived slots, header/body alignment. Mutations ran in a disposable source copy.
- Real CRM server + browser: native drag, insertion line, unchanged geometry before drop, reload persistence, all five table types, light/dark, 900px width and disabled Move Left on the first movable column. Synthetic data only. Evidence: `runs/visual/{before,after,during-drag,dark,narrow-menu,people,companies,deals,follow_ups}.png`, `runs/visual/column-drag.mp4`.
- Pending push migration applied to synthetic Postgres: a real `edit_object` request emitted `changes` with the correct workspace payload. Source review confirms the pending frontend listener invalidates `list_objects`. The notification branch remains separate; deployment verification is read-only.
- Full thermo-nuclear self-review: consolidated header/body/width definitions, fixed computed-column identity collisions, preserved archived slots and existing width preferences, updated the legacy migration snapshot. No remaining structural blocker. Details in `runs/review.md`.
- Merged origin/main 2ea84ca: retained parent-preserving table moves and sidebar positions, regenerated sqlc/MCP assets, renumbered column order to 0058. Fresh full Go suite, vet and build passed; frontend check passed with 26 unit tests plus the native-browser test. Merge review retained both independent ordering fields and no structural blocker.

- Release build follow-up: full browser checks run in GitHub verification with the existing Bun runtime pinned by digest; Docker runs the portable code checks. `list_objects` always returns a columns array, including an empty default. Full local checks re-run after the correction; production verification remains read-only.
- CI browser cleanup: handle CDP pipe errors, reject pending requests when Chrome exits, and await the recorded close even if it happened before cleanup. Full browser check passes; a deliberately failed launch exits in under one second with no remaining profile.
