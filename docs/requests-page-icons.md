# Page Icons

- [x] Add a Notion-style picker with searchable emojis and symbols, pasted emoji support, replacement and removal.
- [x] Save icons on Pages and share rendering across headers, sidebar, breadcrumbs, child-page links, mentions, reference chips, document links and history.
- [x] Migrate existing workspaces without changing their records.
- [x] Complete the requested thermo-nuclear review; preserve custom Icon fields on other objects and complete reference metadata.
- [x] Support images as page icons. The 2026-10-08 correction replaces the previous upload flow with image URLs until object storage is available.
- [x] Remove upload controls, thumbnail conversion and new database image writes. Preserve previously stored images.
- [x] Add saved icon colors, including changing an existing icon's color without reselecting its glyph.
- [x] Expand the searchable catalogs using open-source data and research an open-source Notion alternative.
- [x] Replace rounded filled tabs with square transparent tabs and an active underline.
- [x] Review and verify the final implementation, browser behavior and persistence.

| Before | After |
| --- | --- |
| Rounded, filled tab buttons. | Accessible square tabs with transparent backgrounds and an active underline. |
| A small sample of symbols and emojis. | All 1,854 canonical icons from the installed Lucide library and 3,944 fully qualified Unicode 17 emojis, including skin tones and flags. |
| Icons have one fixed color. | Default plus nine theme-aware colors, saved with the glyph and shared across every icon surface. |
| Image uploads write thumbnails to PostgreSQL. | HTTP(S) image URLs; no upload control or new asset writes. Existing assets remain readable and editable through their displayed URLs. |

The interaction reference is [AppFlowy's open-source picker](https://github.com/AppFlowy-IO/AppFlowy-Web/blob/main/src/components/_shared/icon-picker/IconPicker.tsx). Its catalog has 977 icons across 16 categories; CRM uses its already installed [Lucide](https://lucide.dev/guide/react) catalog rather than copying AppFlowy or Streamline assets. Emoji data comes from [Unicode 17](https://www.unicode.org/Public/17.0.0/emoji/emoji-test.txt); its license is retained beside the generated catalog. No new dependency.

Frontend bun run check, the full sequential Go suite, Go build/vet and sqlc generation pass. Real HTTP/MCP/PostgreSQL checks cover image links, colors, references, history, concurrency preconditions, removal, rejection of uploads and unsafe schemes, legacy assets and workspace isolation. Saving a URL never downloads it on the server. A compiler-overlay control reproduces the legacy URL concurrency failure when boundary normalization is removed.

The real built UI passes mouse selection, color changes, reload, linked-page rendering, URL validation, replacement, broken-image fallback, removal and focus return. Light/dark screenshots were inspected; 390 px iframe measurements confirm picker bounds and no horizontal overflow. The full emoji grid mounts in about 171 ms locally. Keyboard event injection did not reach this browser; synthetic ArrowDown exercises the production focus handler, while the earlier native keyboard checks remain recorded with the original feature.

The embedded app declares HTTP(S) resource origins for user-linked images while keeping network API connections disabled. A browser probe applies the actual MCP resource policy: an image on another origin loads; the previous origin list blocks it. Hosts may further restrict their sandbox policy.

Earlier releases: dc0ed03 added page icons, a06c2b8 corrected the strict-review findings, and 2b60cb8 added the now-replaced upload flow.
