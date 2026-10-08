# Contact and Outreach Fields

- [ ] Remove obsolete contact-level outreach fields and Notion links from the active CAS schema.
- [x] Preserve populated values and history through reversible attribute archiving, with restoration in Settings → Schema.
- [x] Keep Channel, drafts, status and dates on individual Follow-ups/Conversations, allowing multiple outreaches per person.
- [x] Verify archive/restore, active reads/writes/searches, saved filters, history and workspace isolation through production storage and HTTP.
- [ ] Run full checks and strict review; commit/push, verify deployment, then apply and verify the reversible CAS cleanup.

## Implementation and Verification

Custom properties can be archived from their column menu or Settings → Schema. Archive retains their values, sources, history and saved filters; Restore revives them. Active schema, record reads, text search, writes and related records exclude archived properties. Filters depending on an archived property stay stored and become available again on restoration. Required properties expose their protection from the canonical backend policy.

Two PostgreSQL-backed regressions cover the archive lifecycle and MCP schema/reference serialization. A disabled-archive control fails at the active-schema assertion. Real production HTTP routes and PostgreSQL browser fixtures verify Archive/Restore, retained named history and independent Email/LinkedIn follow-ups for one person. Desktop light/dark rendering inspected; narrow Restore rows wrap.

Strict maintainability review passed after correcting incoming archived references and reusing the backend property protection policy. One schema flag and the existing schema edit tool own the lifecycle; no new dependencies, per-contact hiding rules or outreach storage models.

## CAS Cleanup

Pending rollout: archive People.outreach_status, outreach_method, connection_sent_date, message_sent_date, message_sent, last_reply_date, their_reply, next_action, next_action_date and notion_url; archive Companies.notion_url. Preserve contact identity, relationship, Notes, LinkedIn and tags. Individual Follow-ups/Conversations retain Channel, status, dates and drafts. Existing legacy snapshots stay recoverable without inventing historical conversations from incomplete data.
