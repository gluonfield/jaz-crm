# jaz-crm scope

jaz-crm is a CRM that fills itself from email, calendar, meeting transcripts and logged calls. Agents operate it through MCP; people review and correct what it records.

Status: scope draft, 2026-09-30. Nothing is built yet.

## v1

- Google connections per member: Gmail, Calendar, Meet transcripts.
- Backfill and incremental sync.
- Triage that decides which people and companies the CRM keeps.
- One interaction timeline for email threads, meetings, transcripts, and manually logged calls and notes.
- MCP server for agents, with OIDC sign-in, OAuth 2.1 and API keys as in jaz-tasks.
- Workspaces from the first migration.

Later, roughly in order: web record page and timeline, custom objects and attributes (deals), workflows, enrichment, WhatsApp and Telegram, Outlook, sending email and sequences.

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
               title, started_at, ended_at, owner_member_id, visibility)
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

Scopes: `gmail.readonly`, `calendar.readonly`, `meetings.space.readonly`. The Gmail scope is restricted:

- An Internal app on a Google Workspace domain needs no verification.
- An External app in Testing gets refresh tokens that expire after 7 days.
- An unverified External app in production shows a warning screen and is capped at 100 users; wider use needs verification and a security assessment.

### Backfill and incremental sync

- Backfill lists messages over a window (default 2 years) and fetches metadata only: headers and labels. It triages participants, then fetches full bodies only for threads with a kept participant. The page token is checkpointed, so backfill resumes after a restart.
- When a handle becomes kept later, a Gmail search on that address backfills its history.
- Gmail incremental sync calls `users.history.list` from the stored `historyId`; an expired `historyId` triggers a windowed resync. Push runs through `users.watch` → Pub/Sub → `/webhooks/google/pubsub`, renewed daily because watches expire after 7 days. Polling is the fallback.
- Calendar incremental sync calls `events.list` with a `syncToken`; `410 Gone` triggers a full resync. Push runs through `events.watch` channels renewed before expiry, with polling as the fallback.
- Every write is an upsert keyed by `(source, external_id)`, so replays are harmless.

### Triage: who the CRM keeps

Each stage decides what the previous one could not.

1. Rules drop your own addresses, `noreply`-style senders, messages with `List-Unsubscribe` or `Precedence: bulk`, Gmail's Promotions, Social, Updates and Forums categories, calendar rooms and resources, and blocked domains.
2. Engagement keeps two-way contacts: you wrote to them and they wrote to you, or you shared a small meeting. This rule decides most real contacts.
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

- `log_interaction` (MCP tool and UI form) takes a kind, time, participants (email, phone, name or record), title, notes and an optional transcript.
- An uploaded audio file is transcribed by the worker, and the text is attached as parts. The transcription provider is not chosen yet.
- Phone numbers are handles, matched against person phone attributes.

## Workspaces and sharing

- A workspace is the tenant boundary, using the jaz-tasks model: a person has one member row per workspace, invites are the only way in, and every query is scoped by `workspace_id`, with tenant-isolation tests for every endpoint and tool.
- All members of a workspace share its records.
- A connection belongs to one member and has a sharing level for teammates: `full`, `metadata` (who, when, subject) or `private`. The owner can change it for a single interaction.
- One mailbox can feed two workspaces, such as two businesses, as two connections, each triaged against its own workspace description.
- Sub-teams and per-record permissions are not in v1.

## MCP tools (v1)

| Tool | Purpose |
|---|---|
| `search_records` | Find records by text or attribute filters |
| `get_record` | Attributes plus derived stats: last contact, counts |
| `list_interactions` | Timeline for a record, filtered by kind and time |
| `get_interaction` | Full content, subject to sharing |
| `search_interactions` | Full-text search over parts |
| `upsert_record` | Create or update with `source=agent` |
| `log_interaction` | Log a call, meeting or note |
| `link_interaction` | Attach an interaction to a record |
| `list_triage`, `decide_triage` | Review and override keep and skip decisions |

## Architecture

- Stack: Go, Postgres, sqlc, goose, Fx and the MCP go-sdk as in jaz-tasks, plus Temporal.
- `cmd/server` serves the web app, auth, `/mcp` and webhooks. It never calls Google inside a request; webhooks signal workflows.
- `cmd/worker` runs Temporal workflows and activities:
  - `ConnectionSync`: one long-running workflow per connection. It backfills, then waits on push signals or a poll timer, renews watches and channels, and continues-as-new periodically.
  - `HandleHistory`: backfills one newly kept address.
  - `MeetingArtifacts`: a durable timer until the meeting ends, then transcript fetch with retries.
  - `Triage`: batched agent classification.
  - `Transcribe`: turns uploaded audio into parts.
- `internal/google` holds provider clients only, with no storage or workflow knowledge.
- `auth` and `workspaces` start as copies from jaz-tasks. A shared module comes when a third app needs them.

```
backend/
  cmd/server  cmd/worker
  internal/
    app/ auth/ workspaces/
    records/ interactions/ triage/
    google/
    sync/
    httpapi/{mcpapi,authapi,webhooks}
    storage/  storage/postgres/{migrations,queries,generated}
frontend/
```

## Milestones

Ordered by dependency.

1. Foundation: repo, auth and workspaces, records model with people and companies, MCP record tools, tenant-isolation tests.
2. Gmail: connect, backfill, incremental sync, rules and engagement triage, interactions and timeline tools.
3. Calendar and Meet: meeting interactions and transcript fetch.
4. Agent triage, manual interactions, audio transcription, and the inbound interactions webhook.
5. Web record page and timeline (also as a Jaz MCP App), and custom objects and attributes.

## Open decisions

1. Default sharing level for teammates: `metadata` or `full`.
2. Temporal in production: Temporal Cloud or self-hosted.
3. First account: a Google Workspace domain (Internal app, transcripts available) or personal Gmail (unverified-app warning, or 7-day tokens in Testing).
4. Transcription provider for uploaded audio.
