# jaz-crm scope

jaz-crm is a CRM that fills itself from email, calendar, meeting transcripts and logged calls. Agents operate it through MCP; people review and correct what it records.

Status: milestones 1 to 5 are built, 2026-09-30.

## Current request: Companies UI (2026-09-30)

- Use the supplied Attio table as the reference for a clearer Companies table.
- Make categories visible as coloured tags, editable in the table and on company pages.
- Allow several categories per company and reusable custom labels per workspace.
- Filter companies by category and retain text search. Agents use the same category field.
- Verify creation, assignment, removal, filtering, tenant isolation and existing-workspace migration before shipping.

## v1

- Google connections per member: Gmail, Calendar, Meet transcripts.
- Backfill and incremental sync.
- Triage that decides which people and companies the CRM keeps.
- One interaction timeline for email threads, meetings, transcripts, and manually logged calls and notes.
- MCP server for agents, with OIDC sign-in, OAuth 2.1 and API keys as in jaz-tasks.
- Workspaces from the first migration.

Later, roughly in order: workflows, enrichment, WhatsApp and Telegram, Outlook, sending email and sequences.

## Data model

### Taken from Attio

- Objects, attributes, records. People and companies are seeded rows in `objects`, so custom objects (deals, suppliers, quotes) run on the same code path from day one.
- Typed attributes with stable slugs that agents address.
- Append-only values with `active_from`, `active_until` and an actor. One table gives field history, audit, time in stage and "value changed" triggers.
- Identity attributes: person email addresses and company domains are unique per workspace and drive matching.

### Left out

- Lists with list-entry attributes. Attio keeps a second record model beside the first, which is why users ask whether a stage lives on the deal or on the list entry. Here a pipeline is an object with a status attribute, and a list is a saved view: filter, sort, grouping.
- Emails, meetings, notes and calls as separate entities with separate APIs. Here they share one `interactions` model: the timeline is one query, one MCP tool covers every channel, and a new channel is a new `kind`.
- Computed and enriched values stored as attributes. Attio keeps no history for them, so they behave unlike every other attribute. Here stored attributes hold facts someone wrote; last contact, interaction counts and relationship strength are queries over interactions.
- Relationship attributes stored on both sides. Here a reference is stored once and the inverse is an index lookup.

### Added

- Provenance precedence. Each value records its source: `user`, `agent` or `sync`, highest first. A source replaces values from its own rank or lower; a proposed change to a higher-ranked value becomes a suggestion. Agents and sync never overwrite a human edit.

### Storage

```
objects        (id, workspace_id, slug, name, system)
attributes     (id, workspace_id, object_id, slug, name, type, multi, is_unique, config jsonb)
records        (id, workspace_id, object_id, created_at)
record_values  (id, workspace_id, record_id, attribute_id,
                text, number, ts, ref_record_id -> records, json, unique_key,
                active_from, active_until, source, actor_id)
```

- The current value is the row with `active_until IS NULL`, served by partial indexes.
- `unique_key` is set only for unique attributes. A partial unique index on `(workspace_id, attribute_id, unique_key) WHERE active_until IS NULL` enforces identity.
- `ref_record_id` is a real foreign key, so references keep referential integrity.
- Alternatives rejected: JSONB per record (history and uniqueness then need side tables) and a real table per object (runtime DDL and per-workspace schema migrations).
- Filtering and sorting on arbitrary attributes needs a small compiler from typed filters to SQL. sqlc cannot express these queries, so the compiler is the one place outside `queries/` that builds SQL.

### Interactions

```
interactions  (id, workspace_id, kind, source, external_id, connection_id,
               title, started_at, ended_at, owner_member_id)
               unique (workspace_id, source, external_id)
parts         (id, interaction_id, kind, author_handle_id, at, content, content_tsv)
participants  (interaction_id, handle_id, role)
links         (interaction_id, record_id, source)
handles       (id, workspace_id, kind, value, person_record_id, triage, reason, decided_by)
```

- An email thread, meeting or call is one interaction; its messages, transcript entries and notes are parts.
- `kind` is `email`, `meeting`, `call` or `note`, and later `message` for chat.
- `handles` holds every email address and phone number seen, with its triage state.
- Links are written at ingestion. An email to someone at Acme stays linked to Acme after they change company.

## Ingestion

### Connections

A connection is one member's Google account in one workspace, with encrypted OAuth tokens. The CRM owns its connections, so each teammate connects their own account. Jaz keeps its own sync for memory and reads the CRM through MCP.

Google app: the Google Cloud project behind Jaz's Gmail and Calendar connectors. Jaz bundles a desktop client, which only allows loopback redirects, so jaz-crm adds a Web client in the same project with the redirect `{PUBLIC_URL}/connections/google/callback`. Scopes: `gmail.readonly`, `calendar.readonly`, `meetings.space.readonly`; the Meet scope is new to that project's consent screen.

### Backfill and incremental sync

- Backfill lists messages over a window (default 2 years) and fetches metadata only: headers and labels. It triages participants, then fetches full bodies only for threads with a kept participant. The page token is checkpointed, so backfill resumes after a restart.
- Backfill stores metadata for every message, so keeping someone later links their existing threads and fetches the bodies; no per-address backfill is needed.
- Gmail incremental sync calls `users.history.list` from the stored `historyId`; an expired `historyId` triggers a windowed resync. Optional push runs through `users.watch` → Pub/Sub → `/webhooks/google/gmail`, renewed a day before the 7-day expiry; the push is verified as a Google ID token for the configured service account. Polling every five minutes is the fallback.
- Calendar incremental sync calls `events.list` with a `syncToken`; `410 Gone` triggers a full resync. With an https `PUBLIC_URL`, push runs through `events.watch` channels renewed a day before expiry and verified by their token, with polling as the fallback.
- Every write is an upsert keyed by `(source, external_id)`, so replays are harmless.

### Triage: who the CRM keeps

Each stage decides what the previous one could not.

1. Rules drop your own addresses, `noreply`-style senders, messages with `List-Unsubscribe` or `Precedence: bulk`, Gmail's Promotions, Social, Updates and Forums categories, calendar rooms and resources, and blocked domains.
2. Engagement keeps anyone someone in the workspace wrote to, or met, in a conversation of at most 10 participants; an address already on a person record is kept with that person. This rule decides most real contacts, and it overrides rule and agent skips but never a person's decision.
3. An agent classifies the remaining one-way contacts (cold inbound, vendors, recruiters) in batches and returns keep, skip or ask, with a reason. Its criteria come from the workspace description, for example "Acme: manufacturing customers, suppliers, partners".
4. People override decisions in the UI or over MCP. Overrides become address or domain rules that the agent never revisits.

Kept handles become person records, and their domains (freemail excluded) become company records. Skipped handles keep metadata only, so a later override can re-triage them without refetching.

## Meetings and transcripts

- Calendar events become `meeting` interactions with attendees as participants.
- For an event with a Meet link, a workflow waits until the meeting ends. It then finds the conference by meeting code in the Meet REST API, fetches the structured transcript entries and stores them as parts, retrying for up to 24 hours. This covers meetings the user owns or attended, with no subscription to renew.
- Meet deletes transcript entries 30 days after a conference, so the CRM stores its own copy.
- Speakers map to handles through Meet participant records.
- Later, a Workspace Events API subscription on the user, delivered through the same Pub/Sub topic, will cut latency for owned meetings. Invitees receive only `conference.started` and `transcript.fileGenerated`, so calendar-driven lookup stays the primary path.
- Transcripts require transcription to be on in the meeting and a Workspace edition that supports it.

Other recorders (Zoom, Granola, Fireflies) post to `/webhooks/interactions` with an API key, using the same interaction shape.

## Manual interactions

- `log_interaction` (MCP tool and UI form) takes a kind, time, participants (email, phone, name or record), title, notes and an optional pasted transcript.
- Phone numbers are handles, matched against person phone attributes.

## Workspaces and sharing

- A workspace is the tenant boundary, using the jaz-tasks model: a person has one member row per workspace, invites are the only way in, and every query is scoped by `workspace_id`, with tenant-isolation tests for every endpoint and tool.
- Every member sees every record and the full content of every interaction in the workspace. Privacy comes from triage, which stores full content only for kept contacts, and from choosing which mailbox feeds which workspace. Skipping a contact or a thread removes it from the CRM.
- One role, copied from jaz-tasks: an `admin` flag on the member. The workspace creator is an admin, and admins manage invites and members.
- A connection belongs to the member who connected it.
- One mailbox can feed two workspaces, such as two businesses, as two connections, each triaged against its own workspace description.
- Sub-teams, sharing levels and per-record permissions are not in v1.

## MCP tools (v1)

| Tool | Purpose |
|---|---|
| `list_objects` | Objects and their attributes |
| `search_records` | Find records by text or attribute filters |
| `get_record` | Attributes plus derived stats: last contact, counts |
| `list_interactions` | Timeline for a record, filtered by kind and time |
| `get_interaction` | Full content with parts and participants |
| `search_interactions` | Full-text search over parts |
| `upsert_record` | Create or update with `source=agent` |
| `log_interaction` | Log a call, meeting or note |
| `link_interaction` | Attach an interaction to a record |
| `list_triage`, `decide_triage` | Review and override keep and skip decisions for addresses, domains and threads |
| `list_members`, `invite_member`, `rename_workspace` | Workspace membership; admins invite and rename |

## Architecture

- Stack: Go, Postgres, sqlc, goose, Fx and the MCP go-sdk as in jaz-tasks, plus Temporal.
- `cmd/server` serves the web app, auth, `/mcp` and webhooks. It never calls Google inside a request; webhooks signal workflows.
- `cmd/worker` runs Temporal workflows and activities:
  - `ConnectionSync`: one long-running workflow per connection. Each pass runs Gmail backfill and history, Calendar, triage, body fetch, watch renewal and due transcripts as separate activities; a failing stream is logged and skipped, a revoked grant marks the connection and ends it. Passes repeat while work remains, then wait for the poll timer or a wake signal, and the workflow continues-as-new every 100 passes.
  - `MeetingTranscript`: started for each linked Meet meeting that ended in the last 30 days; retries over most of a day for recent meetings, once for older ones, then marks the meeting checked.
- The Temporal client config has `address`, `namespace` and `cloudapikey`. With an API key it connects to Temporal Cloud over TLS; without one it connects to a plain local server. docker-compose runs `temporalio/auto-setup` and the Temporal UI on the same Postgres for development.
- `internal/google` holds provider clients only, with no storage or workflow knowledge.
- `auth` and `workspaces` start as copies from jaz-tasks. A shared module comes when a third app needs them.

```
backend/
  cmd/server  cmd/worker
  internal/
    app/ auth/ workspaces/ errs/
    records/ interactions/ connections/ classifier/
    google/                         # Gmail, Calendar and Meet clients
    worker/                         # Temporal workflows and activities
    httpapi/{mcpapi,authapi,connectapi,webhooks}
    storage/  storage/postgres/{migrations,queries,generated}
frontend/
```

## Milestones

Ordered by dependency.

1. Foundation: repo, auth and workspaces, records model with people and companies, MCP record tools, tenant-isolation tests.
2. Gmail: connect, backfill, incremental sync, rules and engagement triage, interactions and timeline tools.
3. Calendar and Meet: meeting interactions and transcript fetch.
4. Agent triage, manual interactions, and the inbound interactions webhook.
5. Web record page and timeline (also as a Jaz MCP App), and custom objects and attributes.

## Setup before syncing real accounts

- Create the Web OAuth client in Jaz's Google Cloud project and add the Meet scope to its consent screen.
