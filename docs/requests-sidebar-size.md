# CRM Sidebar Size

- [x] Drag the sidebar edge to resize it and give CRM content more room.
- [x] Collapse and reopen the sidebar.
- [x] Remember width and collapse state across visits.
- [x] Preserve mobile navigation and verify desktop, narrow width and both themes.
- [x] Run the requested thermo-nuclear review, reproduce findings and verify the fixes.

| Before | After |
| --- | --- |
| Fixed 232px desktop sidebar. | Draggable edge with bounded width; arrow keys resize and double-click restores the default. |
| Desktop sidebar always visible. | One page-header toggle collapses and reopens at the saved width, preserving keyboard focus. |
| Sidebar size reset on load. | Width and collapse state persist locally. |

Verified against the built app and real HTTP handlers with isolated PostgreSQL: native dragging, width bounds, arrow-key resizing, default reset, collapse/reopen and reload persistence. Light/dark renders and a measured 390px iframe preserve the mobile drawer and show no document overflow. Full frontend and Go checks pass; strict maintainability review keeps shell state in the existing store and uses pointer capture without global drag listeners or new dependencies.

Requested strict review found and reproduced two issues in the initial release: collapse/expand controls removed their focused element, and five pointer moves caused five synchronous storage writes plus a redundant release write. The shared page-header toggle keeps its element across both states; width saves on capture release and explicit keyboard/reset actions.

Review verification: clicking collapse and reopen keeps focus on the same button, while the matched five-move drag makes zero storage writes during movement and one on release. Arrow-key and Home changes persist immediately; full frontend and Go checks pass again.
