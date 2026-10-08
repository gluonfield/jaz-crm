# Page Loading

- [x] Replace the empty header's horizontal line with a normal centered loading spinner when opening pages.
- [x] Use the same loader for record/object schema and route loading, preserving cached content during refresh.
- [x] Verify final revision and strict review.
- [x] Commit/push and verify the deployed interface.

## Interface Review

| Before | After |
| --- | --- |
| Record/page loading rendered an empty bordered header. | A centered 20px spinner replaces the empty header. |
| Object schema loading showed a premature header. | The same spinner waits for the schema. |
| Route loading had no shared pending indicator. | The router uses the same full-height loader. |

## Validation

- Real HTTP/PostgreSQL browser probe: delayed page navigation and actual New Page creation show Loading with zero headers; response completion restores content and New Page title autofocus.
- Cached page content stays visible during a held background refetch.
- Light/dark screenshots inspected; spinner centered exactly in both the full pane and a 360px pane.
- Reduced-motion CSS compiles to a disabled animation. Browser media emulation is unavailable in the integrated browser, so physical reduced-motion behavior was not exercised.
- Strict review: one shared presentation primitive; no new dependencies, state, query policy, or feature-specific loading flags. Four small source files changed; generated embedded app rebuilt normally.

- Final build reloaded in the browser: centered spinner with zero headers in full and dark 360px panes; released response restores the original page content.
- Final frontend check passes (9 tests, TypeScript, ESLint, both builds); full Go test/build/vet passes.

## Rollout

- Implementation `0a09e31dd751c31fe170dc97d8390b645c2e9d08` pushed to main.
- Server `7aec040c-a65c-459a-97e1-e6de591c6532` and Worker `8dafce01-691d-4450-a478-c2a216c78805` report SUCCESS for that commit. CI run 37850217267 verification passed; container publishing continues independently.
- Production browser loaded the new controls asset; SHA-256 exactly matches the local build. Held Company page response shows a centered spinning Loading status and zero headers; release restores the editor. Production checks are read-only.
