# Follow-up Draft Whitespace

Request: Space disappears while editing a saved draft in Follow-ups.

- [x] Reproduce through the real draft editor: adding a trailing space resets “Hello there ” to “Hello there” before any save request.
- [x] Preserve exact input during editing; retain save and revision handling. Reuse the exact field comparison instead of comparing trimmed save payloads.
- [x] Verify the real editor: trailing, repeated and leading spaces; line breaks; independent subject/recipient whitespace; body/subject/recipient saves and record persistence. Undo-edit via the actual Escape handler restores persisted text. After a full reload, the real editor shows the saved body, subject and recipient.
- [x] Frontend `bun run check`: nine tests, typecheck, lint and both builds pass. Backend `go test -p 1 ./...`, `go build ./...` and `go vet ./...` pass. Strict review: the one-line correction reuses the canonical exact-field comparison and preserves the revision guard, save concurrency and sending boundary; no new dependencies, helpers or state.
- [ ] Commit, push and verify the deployment.

Verification used a disposable Postgres database, real HTTP tools and the production frontend in the integrated browser. The scratch owner has no Google connection, so email sending is disabled; edits were saved through the real `save_draft` service. No live drafts were changed and no messages were sent.
