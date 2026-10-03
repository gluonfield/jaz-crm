# Draft quick actions

- [x] Confirm signature behavior: ordinary email includes the Gmail signature in send confirmation and delivery; imported drafts retain existing MIME; non-email Approve adds no Gmail signature.
- [x] Add Shorten, Less salesy, One clear ask, Warmer, Polish and a custom Edit with AI instruction, with compact primary actions and Undo.
- [x] Use one OpenAI SDK response with full stored context, current text and recipients, selected company pages, no model tools, no web retrieval and no truncation.
- [x] Verify latest text/recipients survive delayed results, repeated edits and Undo; show failures and keep sending separately confirmed.
- [x] Run full Go build/vet/tests, frontend checks/build, server startup, live model checks, browser verification and strict review.
- [x] Commit/push and verify production: 1cf3452 is SUCCESS on Server (1a002fcb) and Worker (76f36be2); health, actual composer/menu and a read-only live rewrite pass. The stored record was identical before/after and no mail was sent.

Browser verification covered light/dark themes, a real 390px viewport, imported-draft revision advancement, local/remote edits during rewriting, recipient changes, Undo, errors, duplicate clicks and unmount cancellation. Native keyboard injection was unavailable; keyboard submission was not verified.

Related question: four Kojaddnd entries are four distinct completed sends in one conversation. Each row repeats the latest conversation preview. One current open action waits on them. Proposed presentation is one conversation row with completed actions inside history; no records or messages were deleted or merged. Conversation grouping is separate from these drafting controls.

## Email signature correction

- [x] Replace the initial drafting prompt's instruction to append a sign-off and sender name: generated email ends after its substantive text, with the configured signature added when sending. Quick edits already prohibit adding a sign-off, name or signature and preserve existing user/provider text.
- [x] Real model replies, new emails and quick edits omit closings/names. Restoring the old prompt reproduces “Best, Augustinas”. Full Go build/vet/tests, frontend tests/typecheck/lint and strict review pass; no mail sent.
- [x] Production 33b0831 is SUCCESS on Server (e1af066f) and Worker (9f5211a6); health passes. Existing saved drafts are preserved.
