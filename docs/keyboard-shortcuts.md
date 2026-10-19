# Keyboard Shortcuts

Cmd means Ctrl on Windows/Linux. App actions pause during typing, composition, menus, dialogs and picker interaction. Editors retain their formatting commands. Record actions use the open record or the hovered/keyboard-selected row or board card. The command menu shows the actions available in that context.

## Copy identity

All record kinds, including `pages`, built-in CRM objects and custom table rows, use `CrmRecord.id` at `/r/$recordId`. The same ID is passed to `get_record`, `delete_record`, internal document links and MCP resources (`ui://jaz-crm/r/<id>`). Evidence: `frontend/src/routes/_app/r.$recordId.tsx`, `frontend/src/components/record-table.tsx`, `backend/internal/httpapi/mcpapi/tools_records.go`, and `tools_search.go` (search-result URL = public origin + `/r/` + record ID). Copy ID preserves the complete, case-sensitive string, including historic IDs. No ID is generated or shortened by copying.

Copy link produces `https://crm.jaz.chat/r/<encoded id>`. It deliberately excludes the current origin, search, activity tab and MCP transport URL. The production origin is the live URL named in the request. Both menu actions and shortcut actions call `copyRecord` and show the existing Sonner toast, “Copied”; clipboard failure shows an error.

Tasks' current menu labels are **Copy ID** and **Copy link**. Tasks has no Copy as Markdown link. CRM uses the same labels and bindings. `Cmd+Shift+.` is Tasks' git-branch action; CRM has no git-branch concept and leaves it unbound.

## Mapping to Tasks and Linear

Sources: [Tasks implementation](https://github.com/gluonfield/jaz-tasks/tree/9bede9d1eb8413597ae9ac6d2660306345b8f0de), its `frontend/src/lib/shortcut-definitions.ts`, `components/issue-menu.tsx`, `components/shortcuts-help.tsx` and `docs/keyboard-shortcuts.md`; [Linear reference](https://shortcuts.design/tools/toolspage-linear/), [Linear Search](https://linear.app/docs/search), [Editor](https://linear.app/docs/editor), [Issues](https://linear.app/docs/creating-issues). Checked 19 October 2026. The Tasks thread verified ID/URL/branch keys against Linear's first-party changelogs. Its current-doc corrections (G M for My Issues, Option E to edit, Shift E to estimate) take precedence over the older shortcut reference. Cmd+/ is the shared requested help alias; current Linear public docs did not independently confirm that alias.

| Tasks / Linear keys | CRM action | Result |
| --- | --- | --- |
| Cmd+. | Copy record ID | Added; all record kinds, page or selected row/card |
| Cmd+Shift+, | Copy record URL | Added; canonical link; menu says Copy link |
| Cmd+K | Open command menu | Existing, centralized; toggles its own focused search |
| /, O I | Search records | Global search, matching Tasks; editor slash commands stay local |
| ?, Cmd+/ | Keyboard Shortcuts | Added; same 600px grouped, scrollable dialog and keycaps as Tasks |
| C | Create record | Current object list or open record |
| Enter, O | Open focused record | Existing Enter/O; O waits 800ms for O I |
| Escape | Back / clear selection | Record → object list; list → clear selection; dialogs dismiss locally |
| Up/K, Down/J | Move up/down | Existing, centralized; lists, boards, Triage and Trash |
| Left/Right | Move across board | Added; skips empty columns |
| Cmd+B, G D | Toggle layout / go to board | Added where an object has a pipeline; table/board or follow-up queue |
| G I | Go to Triage | CRM's incoming-contact queue; existing G T retained as alias |
| G M | My follow-ups | CRM's default follow-up queue is scoped to the current workspace member |
| G E | All records | Clear current object's filtering |
| G X | Trash | Replaces CRM's former G X → Connections; Connections remains in the palette |
| G P | People | Existing CRM destination; Tasks uses this for Projects |
| G C | Companies | Existing CRM destination; Tasks has no cycle page |
| G S | Settings | Existing, centralized |
| Option+E | Edit record | Existing record form; open document focuses its editor |
| R | Rename record | Open title or existing edit form |
| S | Status | Existing status picker, when present |
| P, Shift+E | Priority / estimate | Existing property editor, only if the object defines that property |
| A, I | Assign / assign to me | Existing member property, when present |
| L, Shift+L | Change / remove labels | Existing tags picker, where tags are available; selected tags toggle off |
| Shift+D, Cmd+Shift+D | Set / remove due date | Existing due_date or action_date editor; follow-ups use save_action_date |
| Cmd+Option+1–9 | Set status 1–9 | Existing pipeline option order |
| Cmd+Delete | Delete record | Existing confirmation and Trash contract |
| # | Restore selected record | Existing Trash restore API |
| F, Shift+F, Option+Shift+F | Filter / clear last / clear all | Existing filter controls and URL state |
| Cmd+Shift+Up/Down | Open parent / first sub-page | Page hierarchy |
| Cmd+Shift+O | Create sub-page | Existing page creation with parent |
| Option+Shift+Q | Sign out | Web app only; MCP host owns its sign-in |
| Cmd+Enter | Save or submit | Existing record form and composer behavior |
| Cmd+B/I, Cmd+Shift+S, Cmd+E, Cmd+K | Bold / italic / strike / code / link | Existing document editor; editor takes precedence |
| Cmd+Option+1–3; Cmd+Shift+7/8/9; Cmd+Shift+\\ | Headings; todo/bulleted/numbered lists; code block | Existing editor actions, keys aligned to Tasks |
| Cmd+Z / Cmd+Shift+Z | Undo / redo | Existing editor and native controls |
| Y / N | Keep / skip contact | Existing CRM-specific Triage actions, centralized |

## Linear actions without a corresponding CRM feature

| Keys / action | Reason |
| --- | --- |
| Cmd+Shift+. · Copy git branch | CRM records have no branch naming or development workflow |
| Space · Peek; X / Shift+click · bulk select | No record preview or bulk-selection model |
| Cmd+I · Toggle issue details | CRM has responsive details, no visibility toggle |
| G B · Backlog; G V/W · active/upcoming cycle; O C · cycle | No backlog or cycle model |
| G U · Views | Presets exist within each object; no workspace-wide Views page |
| O P/T/F/U/M · project/team/favorite/user/profile | No corresponding switcher or entity pages; people are CRM contacts |
| Ctrl+Shift+1–9 · team navigation | No team-scoped navigation |
| Shift+C/P, Cmd+Shift+C · cycle/project assignments | No cycle/project relations |
| Cmd+Shift+M · move team | Workspace tables are object types, not teams |
| Ctrl+M · comment; Shift+S / Cmd+Shift+S · subscriptions | Conversations are external messages; no internal issue comments or subscription model |
| M B/X/R/M · block/related/merge; Ctrl+L · attachment | No equivalent issue relation/merge/attachment actions |
| Option+C / V · template/full-screen creation | No record templates or separate creation page |
| = / - · timeline zoom | CRM's chronological conversation timeline has no zoom scale |
| Backspace / E · dismiss Inbox update | Triage makes keep/skip decisions; no notification-dismiss operation |
| Cmd+U · Underline; Cmd+Shift+U · upload | No persistent Markdown underline or file-upload feature |

## Shared UI and embedding

The generic platform matcher, scoped dispatcher and help-row layout belong in `@jaz/ui`. Proposed follow-up: extract those primitives once and have both apps consume them; keep app-specific registries and route actions local. The shared dependency stays at its existing pin in this change; no package or cross-repository edit is added.

Full-screen MCP rendering uses the same `_app` routes and AppShell, with in-memory navigation and the host tool transport. Jaz's `MCPAppFrame.tsx` grants `allow="clipboard-write"`. Shortcuts operate while the app iframe has keyboard focus; parent-window shortcuts remain the host's. The hidden browser test also loads the built MCP HTML through the real SDK bridge in an opaque-origin iframe with the same sandbox flags, backed by the synthetic server. It checks the exposed clipboard API, both copy payloads and help. Compact inline record cards render separately and do not register the full app's shortcuts. Hosts that deny clipboard permission receive the copy-error toast.

## Verification

`bun run check` includes the hidden browser suite. It runs the real Go HTTP/auth/tool stack with a new migrated Postgres database, seeds only synthetic records via the authenticated API, and drops that database on completion. Clipboard is the isolated external boundary: the headless fixture records writes in its own JavaScript realm, preserving payload and async failure semantics without changing the person's clipboard. Full-document test navigation disables the back/forward cache to release prior documents and their long-lived streams. Browser processes use a disposable headless profile; no production data or session is used. Screenshots and control evidence live under ignored `runs/shortcuts/`.
