# Document Link Editing — 2026-10-08

- [x] Preserve a URL bullet when Enter creates the next bullet in tested flows.
- [x] Allow editing a link's text and destination from its hover control.
- [x] Preserve record icon rendering, internal navigation, lists and Markdown saves.
- [x] Verify actual mouse edits, destination rejection, reloads, synthetic keyboard shortcuts and undo.
- [x] Complete strict code review and required frontend/Go checks.
- [x] Check light/narrow geometry at a real 760px iframe viewport; inspect the dark form.
- [x] Commit, push and verify deployment.

The reported disappearance has not been reproduced. The correction removes React ownership of the editable link text and disposes its icon renderer. Before the change, replaced links retained old React portals; twelve replacements now leave exactly one portal for one displayed link.

The hover preview opens Text and URL fields through a pencil button. Destination-only edits preserve inline formatting; each edit targets one link and rejects changed document snapshots and unsafe destinations. Cmd/Ctrl+K edits a link under the caret; Search respects an already-handled keyboard event and remains available elsewhere. No new dependencies.

Validation: nine Bun tests, TypeScript, lint and both builds pass; full Go tests, build and vet pass on the final embedded app. A control that expands edits to the whole document fails the bounded-link test. Real mouse edits save and reload while leaving a repeated URL alone. Native keyboard injection still produces no DOM events; keyboard checks use synthetic DOM events. Escape returns focus; Cmd/Ctrl+K opens only the link editor inside a link and only Search elsewhere. The dark form was visually inspected; later screenshot captures time out. At a real 760px iframe viewport, the preview and focused form stay within the viewport and every input/button stays within the form. Editing an external URL into an internal page link updates its record icon and mouse navigation.

Strict review: text DOM ownership is synchronous, icon lifecycle is explicit, edits are atomic, floating controls stay outside the document, and global shortcuts honor event ownership. No structural blockers remain.

Shipped as `c4036eda63bb77e33556f5fb21f236ad45a8b9e2`. Server deployment `85e05bb7-c738-4345-877d-1fdab1c390df` and Worker deployment `6240629f-5ed3-4893-ba15-278796d12e05` both report SUCCESS for that commit. GitHub Actions run `37843409044` has passed its verification job; image publication is still running. Production health returns `ok`; the actual Demos page serves the new synchronous link DOM and displays the hover pencil and correctly populated Text/URL fields. Production checks are read-only. Screenshot capture remains unavailable after the earlier inspected dark form.
