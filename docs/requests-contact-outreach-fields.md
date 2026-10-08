# Contact and Outreach Fields

- [x] Remove obsolete contact-level outreach fields and Notion links from the active CAS schema.
- [x] Preserve populated values and history through reversible attribute archiving, with restoration in Settings → Schema.
- [x] Keep Channel, drafts, status and dates on individual Follow-ups/Conversations, allowing multiple outreaches per person.
- [x] Verify archive/restore, active reads/writes/searches, saved filters, history and workspace isolation through production storage and HTTP.
- [x] Run full checks and strict review; commit/push, verify deployment, then apply and verify the reversible CAS cleanup.

## Implementation and Verification

Custom properties can be archived from their column menu or Settings → Schema. Archive retains their values, sources, history and saved filters; Restore revives them. Active schema, record reads, text search, writes and related records exclude archived properties. Filters depending on an archived property stay stored and become available again on restoration. Required properties expose their protection from the canonical backend policy.

Two PostgreSQL-backed regressions cover the archive lifecycle and MCP schema/reference serialization. A disabled-archive control fails at the active-schema assertion. Real production HTTP routes and PostgreSQL browser fixtures verify Archive/Restore, retained named history and independent Email/LinkedIn follow-ups for one person. Desktop light/dark rendering inspected; narrow Restore rows wrap.

Strict maintainability review passed after correcting incoming archived references and reusing the backend property protection policy. One schema flag and the existing schema edit tool own the lifecycle; no new dependencies, per-contact hiding rules or outreach storage models.

## CAS Cleanup

Applied in CAS: archived People.outreach_status, outreach_method, connection_sent_date, message_sent_date, message_sent, last_reply_date, their_reply, next_action, next_action_date and notion_url; archived Companies.notion_url. Preserve contact identity, relationship, Notes, LinkedIn and tags. Individual Follow-ups/Conversations retain Channel, status, dates and drafts. Existing legacy snapshots stay recoverable without inventing historical conversations from incomplete data.

## Rollout

Implementation `31079d707cf5b175e6dc997b9ca9273dd4a4a4f0` is pushed to main and live on both services (Server `579ba4a3-0422-4cc9-9a3e-11d99b46bbe2`, Worker `e7b45420-1fbc-4ecf-847b-b72ab83f36f5`). CI verification passed. All eleven CAS properties are archived. Production active schema and actual contact Details are clean; record counts and sampled People/Companies histories and retained field values are unchanged. The live Settings page exposes all eleven Restore controls; its served schema bundle matches the locally verified SHA-256 `401ca2fd4d4bdc5bf56d9d420996e75f93c53856208e3405be5754c4f855463f`. Health returns 200.

| Before | After |
| --- | --- |
| Legacy outreach and Notion properties appeared on contacts | Eleven obsolete CAS properties archived; contact identity, relationship and notes remain active |
| Property removal was permanent | Existing property menu and Settings → Schema support Archive/Restore |
| Built-in properties offered unavailable destructive actions | Menu actions use the backend's canonical protection policy |
| Retired values could affect search and related records | Active reads, text search, writes and incoming references exclude archived properties |
| Saved views relied on retired properties | Such views are hidden while archived and return intact on restoration |
| History labels relied on the active schema | Changes resolves readable names from the complete schema |
| Restore actions could overflow at narrow widths | Rows wrap and all three fixture actions stay inside a 390 px viewport |
