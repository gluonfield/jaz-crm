# Page icons

- [x] Add a Notion-style page icon picker with searchable emojis and symbols, pasted emoji support, replacement and removal.
- [x] Save icons on the page and show them in the page header, sidebar, child-page links and document links.
- [ ] Verify existing-workspace migration, MCP persistence, actual UI and full checks; review, commit and push.

| Before | After |
| --- | --- |
| Pages use a fixed document glyph. | Choose or remove an emoji or symbol from the title/header; the sidebar, breadcrumbs, sub-pages, mentions, reference chips and document links share the saved icon. |
| Existing workspaces have no page icon attribute. | Migration 43 adds Icon while preserving existing records; new workspaces include it. |

Frontend `bun run check`, Go build/vet and the full Go suite (`go test -p 1 ./...`) pass. Real HTTP/MCP/storage checks cover emoji/symbol replacement, complex emojis, removal, reference updates and preserved titles/content. Initial parallel backend tests failed outside this feature; sequential package verification passes. Code review found no structural regression or new dependency.

Dark-mode picker inspected in the real page. The side browser disconnected during an interruption, leaving final interaction/reload/light/narrow checks pending.
