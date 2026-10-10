# List motion

- [x] Animate completion and removal in Follow-ups, including the final row in a group.
- [x] Close gaps smoothly in record lists, tables, boards and Triage; animate restored rows leaving Trash.
- [x] Keep motion brief, interruptible and disabled for reduced motion, without entrance choreography on loading or navigation.
- [x] Verify real UI interactions, both themes, narrow layouts, rapid changes and failed writes.
- [x] Review and run full checks.
- [x] Commit and push.
- [x] Run the requested thermo-nuclear review and resolve blocking findings.

One rendering boundary measures rows in React's snapshot lifecycle before they move or unmount. Remaining rows slide from their painted positions; removed rows fade and lift 4px as inert, inaccessible visual copies. All motion takes 160ms, with no bounce or stagger. Snapshots preserve table column widths and scroll clipping. Nested boards own their card movement; stable record IDs preserve identity between columns.

This replaces the old offset-only hook, deletion CSS and Triage's height-collapse CSS. Tool requests no longer wait for animation timers. Done and deletion actions prevent repeated submission while saving.

Verification used the built web app and production HTTP/storage code with a disposable PostgreSQL database. Browser checks covered completion, failed completion and retry, the last item in a group, two removals 40ms apart, failed Triage recovery, card movement between columns, table column alignment, restoration of the last Trash item, light/dark themes and a measured 390px iframe. Painted-frame samples confirm continuous movement; reduced-motion input produces zero list animations. Temporary visual copies disappear and browser error capture remains empty. Frontend tests (18), TypeScript, ESLint, web/MCP builds and Go build/vet/tests pass. Strict review found no remaining blockers. Scratch files and logs stay in gitignored `runs/list-motion/`.

The requested follow-up review found repeated layout work in the per-row read/write loop. Cancellation, measurement and animation now run in separate passes. A browser fixture importing the production component measured one removal from a 1,000-row, 12-column table at a median 1,667ms before and 94ms after (three trials each); 100 rows fell from 31ms to 6ms. These are local stress measurements, including React's synchronous update. Interrupted removals preserve the same row's painted position exactly; scope changes, unmount and reduced motion leave no active animations or visual copies. Full frontend and Go checks pass again. No remaining structural blockers, dependencies or new state; verification files stay in `runs/motion-review/`.

| Before | After |
| --- | --- |
| Follow-ups completion and filtered removals jumped. | Departing rows fade, and remaining rows and group headings close the gap. |
| Board cards changed columns instantly. | Cards move between columns under their record identity. |
| Triage, deletion and Trash used different or missing exits. | They share brief exits and smooth row movement. |
| Animation timing delayed tool requests. | Requests settle normally; the rendering boundary owns motion. |
| Done and deletion could be submitted repeatedly while saving. | Their actions are disabled until the write settles. |
