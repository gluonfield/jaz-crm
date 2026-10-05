# Linked records in documents

- [x] Identify why linked competitor records show @ and lack logos: a CSS prefix and plain link rendering without record metadata.
- [x] Show company logos, person avatars and page icons through the existing RecordIcon component in editable and read-only markdown.
- [x] Preserve document text, Markdown links, navigation and external-link attributes; verify editing, autosave and round-tripping through the real browser, editor and HTTP API.
- [x] Review and run full frontend/backend checks before committing and pushing.

Work is isolated from the concurrent product-ID migration in the main checkout. No new dependency, backend endpoint or persisted document format is needed.

The existing Link mark gets a React mark view in the shared document StarterKit. Record metadata uses the existing query cache, and icons use RecordIcon; ordinary links retain their attributes. Missing and broken logos keep the standard fallback. Rendering is a view only: markdown still stores labelled links. The actual page and follow-up context surfaces were checked against production handlers and disposable PostgreSQL storage, including repeated/bold links, cached lookup deduplication, zero writes on initial rendering, editor-command changes and persisted autosave, and native navigation through a linked page. Native key injection was unavailable: the same insertion attempt left a plain textarea unchanged, so the edit check uses the real Tiptap transaction and autosave path instead.

Strict review found no remaining issues. Full frontend check and Go build, vet and test passed after rebasing onto the product-ID migration. Rendered light/dark and 700px layouts preserve inline alignment and show loaded logos without horizontal overflow. The disposable browser fixture was stopped and its database cleaned up.

| Before | After |
| --- | --- |
| Every CRM document link had an @ prefix and no record icon. | Links show the existing company logo, person avatar or page icon, with reserved inline spacing and the normal fallback. |
| Editable/read-only documents used plain link mark views. | Both surfaces share the same record-aware mark view; links keep their content and attributes. |
