# Follow-ups and drafts

- [x] Track next steps for any relationship (customers, advisers, investors, reconnects) as a standard Follow-ups object: action, status, waiting on us/them, review date, owner, person, company, deal.
- [x] Keep an optional draft on each follow-up: draft, channel (Email/LinkedIn), To, Cc, draft status.
- [x] Only a person approves a draft; an approved draft is claimed for sending once (Sending) and then marked Sent; editing withdraws approval.
- [x] Reuse existing record tools (`upsert_record`, `search_records`, `get_record`, saved filters) instead of new follow-up tools.
- [x] `get_record` returns the records that reference it, including follow-ups; `search_records` sorts by a date such as `review_on`.
- [x] `log_interaction` records LinkedIn conversations as messages under a stable `external_id`; the sender marks a draft Sent with `upsert_record`.
- [x] Send email drafts from the CRM as replies in the linked thread, from the mailbox holding the latest message (any teammate's unless they opt out), refusing drafts older than the latest message. Needs a Google reconnect for `gmail.send`.
- [ ] Follow-up agent on `gpt-6-luna`, medium effort, through the official OpenAI Go SDK: reads each new message in or out, updates follow-ups, drafts replies only when useful.
- [ ] Review queue: Needs attention view with Send (email) and Approve (LinkedIn).
- [ ] Gmail's own drafts are not imported as sent mail.
- [ ] Commit, push and check local and Railway activation.
