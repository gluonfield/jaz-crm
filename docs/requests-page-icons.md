# Page icons

- [x] Add a Notion-style page icon picker with searchable emojis and symbols, pasted emoji support, replacement and removal.
- [x] Save icons on the page and show them in the page header, sidebar, child-page links and document links.
- [x] Verify existing-workspace migration and MCP persistence; commit and push the page-icon feature.
- [x] Complete browser interaction/reload checks and light/narrow layout measurements after the side browser reconnects.
- [x] Complete the requested thermo-nuclear review, correct findings and verify corrections.
- [x] Allow uploaded images as page icons, with replacement/removal and shared rendering; review and verify.

| Before | After |
| --- | --- |
| Pages use a fixed document glyph. | Choose or remove an emoji or symbol from the title/header; the sidebar, breadcrumbs, sub-pages, mentions, reference chips and document links share the saved icon. |
| Existing workspaces have no page icon attribute. | Migration 43 adds Icon while preserving existing records; new workspaces include it. |
| Page icons accept emojis and symbols. | The Image tab uploads, replaces and removes proportion-preserving thumbnails; shared page surfaces and history render them. |

The feature passes frontend `bun run check`, Go build/vet and the full Go suite (`go test -p 1 ./...`). Real HTTP/MCP/storage checks cover emoji/symbol replacement, complex emojis, removal, reference updates and preserved titles/content. No new dependency.

The reconnected side browser exercised actual PNG/JPEG uploads, invalid-file rejection, replacement, reload, removal, linked-page icons and keyboard emoji/symbol selection with natural focus return. Images retain transparency and become 128 × 128 thumbnails; tool reads carry URLs. Light/dark computed styles and 390 px iframe layout pass, including picker bounds. The normalized thumbnail was visually inspected; full-page screenshot capture times out, so screenshot review remains unavailable.

The requested review corrected two issues: page-only metadata filtering hid custom Icon fields in cards and related-record details; three backend callers reconstructed the same reference metadata. Icon filtering now applies only to Pages, and the existing resolver returns complete reference values. Real rendering regressions fail before the correction and pass afterward. MCP/storage coverage preserves an untitled page's reference ID after removing its only icon, with a failing compiler-overlay control.

Review corrections pass the full frontend check, sequential Go suite and Go build/vet. No further structural blockers found; changed handwritten files remain below 1,000 lines.

Image storage and record changes share one transaction. Real HTTP/MCP/PostgreSQL checks cover thumbnail pixels, references, idempotent saves, independent copies, replacement/removal, history, deletion, rejected uploads, cross-workspace reuse and rollback. The final frontend check, full Go suite and Go build/vet pass. No new dependency.
