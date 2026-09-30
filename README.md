# Jaz CRM

A self-hosted CRM that fills itself from Gmail, Google Calendar and Meet transcripts, and that agents operate over MCP. People, companies and any objects you add keep every value's history and who set it; each person's and company's page shows the emails, meetings, calls and notes with them.

## Run it

```sh
cp .env.example .env   # set OIDC_*, GOOGLE_*, ENCRYPTION_KEY
docker compose up
```

This starts Postgres (host port 55532), Temporal (7533, UI on 8533), the server on http://localhost:7500 and the sync worker. Sign in, open Connections and connect a Google account; its mail and calendar sync in the background.

Agents connect to `/mcp` with OAuth, or with an API key as a bearer token:

```sh
claude mcp add --transport http jaz-crm http://localhost:7500/mcp --header "Authorization: Bearer $OWNER_API_KEY"
```

## How it works

- **Records.** Objects are record types; every workspace starts with people, companies and deals and can add its own, such as suppliers. Attributes are typed: text, number, date, checkbox, url, select, status, email, domain, phone and reference. A status is a select whose options are ordered stages, so an object with one is a pipeline, shown as a board. Emails, domains and phone numbers identify records, so writing a known email updates that person instead of creating another.
- **Companies.** Assign several coloured category tags directly in the table or on a company page. Create reusable labels, filter by a category, search and sort by name; company names stay visible while the table scrolls.
- **History and provenance.** Values are append-only: a change closes the current value and inserts its successor. Each value records its source, ranked user > agent > sync; a write replaces or removes only values from its own or a lower-ranked source and reports the rest as skipped, so sync never overwrites an agent and neither overwrites a person.
- **Interactions.** An email thread, meeting, call or note is one interaction with its participants and parts: messages, transcript lines, notes. Threads seen by two teammates' mailboxes merge by Message-ID. Interactions link to the records they concern, and a record's timeline is one query.
- **Triage.** Every address seen is triaged. Your own addresses and colleagues are internal; automated senders and bulk mail are skipped. Anyone someone in the workspace wrote to, or met in a small meeting, is kept and becomes a person at their company with their conversations linked. An optional LLM judges cold inbound against the workspace's description; the rest wait for a person or an agent. A person's decision, for an address or a whole domain, is final.
- **Privacy.** Metadata is stored for every message, but bodies are fetched only for conversations linked to a kept person or a record, and forgotten when they no longer are. Skipping a thread removes it for good.
- **Sync.** One Temporal workflow per connection backfills mail and calendar (`BACKFILL_DAYS`), then follows Gmail history and Calendar sync tokens every five minutes, or at once on a push. Meet transcripts are fetched after meetings end and kept, since Meet deletes them after 30 days.
- **Web app.** Triage, conversations, every object as a table, and a page per record with its fields, timeline and a form to log a call or note; ⌘K searches records. In Jaz the same app opens from the sidebar as an MCP App through the `show_crm` tool.
- **Calls and recorders.** Log a call or note in the app or through `log_interaction`. Recorders post conversations to `POST /webhooks/interactions` with an API key; an `external_id` makes a repeated post update one interaction.

## Authentication

- **People** sign in with OpenID Connect. Anyone with a verified email may sign up unless `ALLOWED_EMAIL_DOMAINS` or `ALLOWED_EMAILS` restrict it. Each new person gets a workspace; invites are the only way into another. Every member sees the whole workspace; admins invite people and edit the workspace.
- **Agents** use OAuth 2.1 with dynamic client registration and PKCE, discovered from the 401 on `/mcp`. Personal API keys work as bearer tokens too. The web app calls the same operations at `POST /api/tools/{tool}` with its session.
- **Tenant isolation**: every query and tool is scoped to one workspace; tests attack another workspace's records and interactions by id.

Configuration is documented in `.env.example`.

## MCP

| Tool | Does |
| --- | --- |
| `list_objects`, `create_object`, `create_attribute` | the schema |
| `add_attribute_option` | reusable select choices, including company categories |
| `search_records`, `get_record`, `upsert_record`, `delete_record` | records; `get_record` includes how often and when last they were in touch |
| `list_interactions`, `get_interaction`, `search_interactions` | timelines, full conversations, full-text search |
| `log_interaction`, `link_interaction`, `unlink_interaction`, `skip_interaction` | calls and notes, links to records, removal |
| `list_triage`, `decide_triage` | who is kept, skipped or waiting, and decisions by address or domain |
| `list_connections`, `disconnect` | synced Google accounts |
| `get_workspace`, `update_workspace`, `invite_member` | the workspace, its description and members |
| `show_crm` | opens the web app in hosts that support MCP Apps |

## Develop

```sh
docker compose up -d postgres temporal
cd backend
go build ./... && go vet ./... && go test ./...
sqlc generate                      # after changing SQL in internal/storage/postgres/queries
go run ./cmd/server & go run ./cmd/worker
cd ../frontend && bun install && bun run dev
```
