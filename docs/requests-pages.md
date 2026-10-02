# Pages and tables

- [x] A Data section in the sidebar with + to create a Page or a Table.
- [x] Tables are workspace objects; columns have types, including references to people, companies, deals, pages or other tables.
- [x] Clicking a row opens it as a page with a WYSIWYG markdown editor.
- [x] Standalone pages nest from the start: sub-pages, a sidebar tree, breadcrumbs and moving pages.
- [x] Typing @ in a page mentions a person, company or page by title, with its path shown, as a clickable link.
- [x] Rename and delete tables and columns.
- [x] Every page adds a page inside it from an Add page button, so a Knowledge Base page can hold Vision, Strategy and Team.
- [x] Pages look like Notion: titles are borderless headings that wrap, and a page lists its pages as plain links.
- [x] Columns are added as in Notion: + after the last column opens a name field and a list of types with icons, including links to people, companies, pages or tables. A new table starts with one untitled row and opens as a table; a status added later starts existing rows in its first stage.
- [x] A table's records open as pages: the title, then the record's properties with their type icons and Add a property, then its content; no conversations, changes or details panel.
- [x] Agents read and write all of it through MCP: tables, columns, rows, page content, nesting and mentions.
- [x] Review, verify real PostgreSQL/MCP persistence and the actual UI, commit/push and activate locally and on Railway.

A table is a workspace object and its records are pages: every object a workspace creates has `name` and markdown `content`. Standalone pages are the standard `pages` object, nested through a single-valued `parent` reference to itself. A single-valued self-reference refuses cycles. A mention is a markdown link to `/r/<record id>`, so agents read and write mentions as ordinary links.

Content is one record value, so history and source attribution apply. One member's edits within ten minutes revise one version instead of adding one per autosave, and any writer replaces it whole. Writes may `expect` the content they read; the editor always does, so it never overwrites an agent's change it has not seen and shows the latest version instead. Lists leave content out, while text search still matches it.

The editor is Tiptap 3.31.4 (MIT) with its official markdown package, approved on 2026-10-02. Saving rewrites agent markdown into Tiptap's normal form (`*` emphasis, padded tables, explicit autolinks); the meaning is unchanged and a second round trip is identical.

Deleting a page moves its pages to the top level. Deleting a table deletes its records and the reference columns pointing at it, and drops those columns' saved-filter conditions.

Verification: Go build, vet and the full suite pass, including service, migration and MCP tests with negative controls for the cycle check, conflict guard, revisions, list content and filter cleanup. Frontend typecheck, lint and build pass. On a scratch server with agent-seeded data, headless Chrome confirmed the Data tree, breadcrumbs, title and path mentions, saving, opening a mention, new pages and sub-pages, Move to, new tables, an added Companies column, inline cells, collapsed history, the agent-edit conflict, light and dark themes, and a 760px width. `876586d` runs locally (migration 23) and on Railway Server and Worker. A read-only production probe lists Pages with name, parent and content.
