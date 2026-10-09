# Follow-ups Redesign

Source: Augustinas, 2026-10-08 to 2026-10-09. The Follow-ups queue looked confusing; a redesign was proposed from real CRM cases and approved for implementation.

- [x] Group the queue by whose move it is: To do, Waiting on them (until the chase date comes), Done and Dismissed.
- [x] Give each row one due label and a Reply or Chase tag instead of status text, colours and avatar dots.
- [x] Lead the detail with the task and its due date; move the person, their context and the conversation's steps into a side panel on wide screens.
- [x] Show email as a mail reader: latest two messages open, earlier ones one line each, a long middle folded behind "N more messages". LinkedIn stays a chat.
- [x] Show the meeting or call a follow-up came from when it has no message thread.
- [x] Compact the composer to one recipients line, with rewrite actions and Send in one row; show why no draft was written above the reply box.
- [x] Keep the reply box on every follow-up so Enter always edits it; say who we are waiting for and the chase date above it.
- [x] Keep every linked action of a conversation reviewable and selectable, including done and dismissed ones, on wide and narrow screens.
- [x] Verify with a seeded full stack (long email thread, LinkedIn chase, skipped draft, meeting task, waiting) in dark, light and narrow layouts; run full checks and strict review.

- [x] Run the requested thermo-nuclear review: one standing model and due rule shared with the server's Chase filter, one text helper, one date-of-today helper, an always-present reply box, one details panel, parallel source loading, and no duplicated recipients in the composer.
- [x] Reduce the queue header to one row: the view as its title, a filter icon that counts only added conditions, a one-word owner switch and a search icon that expands on click or /. Sorting stays in the table view.
- [x] Draft a first message only on request: an empty reply box offers Draft with AI, which uses the existing AI edit call with a write action. Nothing is generated in the background. A chase becomes a short nudge; facts or choices only the sender knows become bracketed placeholders, and no sign-off is added.
- [x] Second requested thermo-nuclear review: one owner switch design in both views, filter counts and view labels measured against the view's defaults in both views, and a named rule for when AI may propose a subject.
- [x] Make Draft with AI a split button: the main part drafts what the follow-up needs; its menu offers Nudge, Accept, Decline, Suggest a call and Tell AI what to write, sharing one menu component with Edit with AI.
- [ ] Meeting summaries with the agreed next step.

The seeded stack is a disposable PostgreSQL workspace driven through the real tools API and a headless Chrome over CDP, because the side browser could not capture this page. Checks covered folding and expanding mail without the view jumping, focus moving to revealed messages, Enter focusing the reply box on waiting and task follow-ups, switching to done and dismissed steps, the narrow details panel and the standalone conversation page. Sending was not exercised: the harness has no Gmail connection, and send logic is unchanged.
