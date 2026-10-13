# CRM Sidebar Size

- [x] Drag the sidebar edge to resize it and give CRM content more room.
- [x] Collapse and reopen the sidebar.
- [x] Remember width and collapse state across visits.
- [x] Preserve mobile navigation and verify desktop, narrow width and both themes.

| Before | After |
| --- | --- |
| Fixed 232px desktop sidebar. | Draggable edge with bounded width; arrow keys resize and double-click restores the default. |
| Desktop sidebar always visible. | Header collapse button; page-header button reopens at the saved width. |
| Sidebar size reset on load. | Width and collapse state persist locally. |

Verified against the built app and real HTTP handlers with isolated PostgreSQL: native dragging, width bounds, arrow-key resizing, default reset, collapse/reopen and reload persistence. Light/dark renders and a measured 390px iframe preserve the mobile drawer and show no document overflow. Full frontend and Go checks pass; strict maintainability review keeps shell state in the existing store and uses pointer capture without global drag listeners or new dependencies.
