# Notion-like Page Editor

Source: Augustinas, 2026-10-16; screenshot of a `/` block menu (Heading 1–3, Bulleted list, Numbered list, Checklist, Quote, Code, Math, Table, Divider) and a block drag handle.

- [x] `/` opens a filterable block menu with markdown shortcut hints; it turns the current line into a heading, list, checklist, quote or code block, or inserts a table or divider.
- [x] Hovering a block shows `+` and a drag handle. `+` starts a line below with the `/` menu open; dragging moves the block; a click opens Turn into, Duplicate and Delete. List items move one at a time.
- [x] Selected text shows a format bar: bold, italic, strikethrough, code and link; ⌘K links a selection.
- [x] A handle in the current table cell inserts and deletes rows and columns.
- [x] Placeholders name the empty line being typed on: commands hint, Heading 1–3, List, To-do.
- [x] Follow-up (screenshot of a Notion page): Notion's document rhythm and checkboxes. Text is 16px on 24px lines with 16px between blocks and 6px between list items, matching the 40px and 30px baseline pitches measured from the screenshot; outlined 16px checkboxes fill with the accent and a white check; titles are 40px bold with Notion's gap to the text; headings, quotes, code, tables and dividers use Notion's proportions.
- [x] Follow-up: square checkboxes.
- [x] Follow-up (Notion `/` menu screenshot): Callout blocks with a changeable icon and color. The icon uses the page icon picker (emoji, icon or image, or none); Color sits in the block menu with the page icon palette. Markdown stores a callout as an Obsidian-style quote, `> [!callout|blue] 💡` then its body, which other renderers show as a quote; the agent tool description documents it.
- [ ] Math blocks: need `katex`, a new dependency awaiting approval.

Pages still store markdown; every block the menu creates saves and reloads through the existing format. `/` stays out of tables and code because markdown table cells hold only text.

Verification: tests cover each `/` conversion from a line and from a list item, empty-line inserts, and handle actions on a nested checklist, each checked against a markdown reload; negative controls confirm they fail when the conversions break. The real app with an isolated backend confirmed the menu, keyboard selection, drag reordering of list items and top-level blocks, caret restoration after a drop, Turn into, Duplicate, Delete with undo, the format bar and link entry, table row and column edits, mentions, placeholders, and light and dark themes. The page column gained a left gutter so handles stay visible beside narrow content; touch screens hide them.

Strict review (2026-10-16): each suggestion menu now renders through Tiptap's `ReactRenderer` inside its own extension, which deletes the React state bridge and the page editor's menu wiring; the `/` menu is a module-level extension. Table detection uses ProseMirror's `cellAround`, which fixes `/` opening in a list inside a table cell (the old parent check missed it), and the table menu follows one transaction listener instead of four events. Known limit: a numbered item dragged out of its list becomes a bullet, because ProseMirror wraps a lone item in the first list type. Carrying the list with the item kept its type but split text when dropped mid-line, so it was reverted.
