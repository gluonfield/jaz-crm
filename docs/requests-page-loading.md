# Page Loading

- [x] Replace the empty header's horizontal line with a normal centered loading spinner when opening pages.
- [x] Use the same loader for record/object schema and route loading, preserving cached content during refresh.
- [x] Verify final revision and strict review.
- [ ] Commit/push and verify the deployed interface.

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
