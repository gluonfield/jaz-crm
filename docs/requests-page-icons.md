# Page icons

- [x] Add a Notion-style page icon picker with searchable emojis and symbols, pasted emoji support, replacement and removal.
- [x] Save icons on the page and show them in the page header, sidebar, child-page links and document links.
- [x] Verify existing-workspace migration and MCP persistence; commit and push the page-icon feature.
- [ ] Complete final browser interaction/reload/light/narrow checks after the side browser reconnects.
- [x] Complete the requested thermo-nuclear review, correct findings and verify corrections.

| Before | After |
| --- | --- |
| Pages use a fixed document glyph. | Choose or remove an emoji or symbol from the title/header; the sidebar, breadcrumbs, sub-pages, mentions, reference chips and document links share the saved icon. |
| Existing workspaces have no page icon attribute. | Migration 43 adds Icon while preserving existing records; new workspaces include it. |

The feature passes frontend `bun run check`, Go build/vet and the full Go suite (`go test -p 1 ./...`). Real HTTP/MCP/storage checks cover emoji/symbol replacement, complex emojis, removal, reference updates and preserved titles/content. No new dependency.

Dark-mode picker inspected in the real page. The side browser disconnected during an interruption, leaving final interaction/reload/light/narrow checks pending.

The requested review corrected two issues: page-only metadata filtering hid custom Icon fields in cards and related-record details; three backend callers reconstructed the same reference metadata. Icon filtering now applies only to Pages, and the existing resolver returns complete reference values. Real rendering regressions fail before the correction and pass afterward. MCP/storage coverage preserves an untitled page's reference ID after removing its only icon, with a failing compiler-overlay control.

Review corrections pass the full frontend check, sequential Go suite and Go build/vet. No further structural blockers found; changed handwritten files remain below 1,000 lines.
