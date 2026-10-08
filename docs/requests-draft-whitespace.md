# Follow-up Draft Whitespace

Request: Space disappears while editing a saved draft in Follow-ups.

- [x] Reproduce through the real draft editor: adding a trailing space resets “Hello there ” to “Hello there” before any save request.
- [x] Preserve exact input during editing; retain save and revision handling. Reuse the exact field comparison instead of comparing trimmed save payloads.
- [x] Verify the real editor: trailing, repeated and leading spaces; line breaks; independent subject/recipient whitespace; body/subject/recipient saves and record persistence. Undo-edit via the actual Escape handler restores persisted text. After a full reload, the real editor shows the saved body, subject and recipient.
- [x] Frontend `bun run check`: nine tests, typecheck, lint and both builds pass. Backend `go test -p 1 ./...`, `go build ./...` and `go vet ./...` pass. Strict review: the one-line correction reuses the canonical exact-field comparison and preserves the revision guard, save concurrency and sending boundary; no new dependencies, helpers or state.
- [x] Committed/pushed `8c87d2b`. Server deployment `04da3767-d4d1-4a1c-bc82-e1b59bb70fee` and Worker deployment `ed6454f1-b8e8-432d-a4e9-df550d47a9f9` both report SUCCESS for that exact commit. GitHub Actions run `37845283024` has passed its verification job. Read-only production health returns `ok`; the served Draft component contains the exact-field comparison and its bundle SHA-256 matches the tested local build (`36e2cfea41f66549f1181d44f9962ab13798149b38974b587b846b4c3b0f5e23`).

Verification used a disposable Postgres database, real HTTP tools and the production frontend in the integrated browser. The scratch owner has no Google connection, so email sending is disabled; edits were saved through the real `save_draft` service. No live drafts were changed and no messages were sent.
