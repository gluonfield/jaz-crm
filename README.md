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

## Container images

GitHub Actions verifies the backend and publishes two images for Linux amd64 and arm64:

- `ghcr.io/gluonfield/jaz-crm-server`: HTTP, MCP and the web app; listens on port 7500.
- `ghcr.io/gluonfield/jaz-crm-worker`: background sync; starts the worker directly, with no web assets or HTTP port.

Main publishes `latest` and the full commit SHA. A `v*` Git tag publishes that tag and its commit SHA. Pin both services to the same SHA or release tag for a repeatable deployment.

Copy `.env.example` to `deployment.env`. Set `DATABASE_URL` to a reachable Postgres database, `TEMPORAL_ADDRESS` and `TEMPORAL_NAMESPACE` to your Temporal service, and `PUBLIC_URL` to the public server URL. Add `TEMPORAL_API_KEY` for Temporal Cloud. Configure `OIDC_*` for sign-in and `GOOGLE_*` plus a stable `ENCRYPTION_KEY` for sync. Both containers use the same database, Temporal settings and encryption key; configuration is read at startup.

```sh
docker run -d --name jaz-crm-server --restart unless-stopped \
  --env-file deployment.env -p 7500:7500 ghcr.io/gluonfield/jaz-crm-server:latest
docker run -d --name jaz-crm-worker --restart unless-stopped \
  --env-file deployment.env ghcr.io/gluonfield/jaz-crm-worker:latest
```

Postgres and Temporal run separately. Schema migrations run automatically. `GET /healthz` checks the server; worker startup and sync are reported in its logs. The images run as a non-root user. GitHub creates new packages as private: authenticated pulls need a token with `read:packages`, or the package owner can make each package public in its settings.

Build locally with `docker build --target server -t jaz-crm-server .` and `docker build --target worker -t jaz-crm-worker .`. Docker Compose selects these targets automatically.

## How it works

- **Records.** Objects are record types; every workspace starts with people, companies and deals and can add its own, such as suppliers. Attributes are typed: text, number, date, checkbox, url, select, status, email, domain, phone and reference. A status is a select whose options are ordered stages, so an object with one is a pipeline, shown as a board. Add stages, drag their grips to reorder, or click a grip to rename, move, collapse or delete a stage. Deleting moves its deals to a chosen stage and keeps their history. Emails, domains and phone numbers identify records, so writing a known email updates that person instead of creating another.
- **Companies.** Assign several coloured category tags directly in the table or on a company page. Create reusable labels, filter by a category, search and sort by name; company names stay visible while the table scrolls.
- **History and provenance.** Values are append-only: a change closes the current value and inserts its successor. Each value records its source, ranked user > agent > sync; a write replaces or removes only values from its own or a lower-ranked source and reports the rest as skipped, so sync never overwrites an agent and neither overwrites a person.
- **Interactions.** Notes have one text body and author. Message threads carry a channel and attributed messages with recipients and original dates. Calls and meetings can carry notes and speaker turns. Source details stay separate in provenance. Threads seen by two teammates' mailboxes merge by Message-ID. Interactions link to records; notes appear on the timeline without counting as contact.
- **Triage.** Contacts wait for approval by default. Settings independently opt into keeping people you email, people from completed small meetings, existing CRM contacts, or AI decisions against your Who belongs criteria. Your own addresses and colleagues are internal; automated senders and bulk mail are skipped. In Settings, members list other addresses they send from, such as a university mailbox (admins can list anyone's), so that mail reads as the workspace's own without making the rest of that domain internal. Explicit address and domain decisions take precedence. Deleting a company excludes its domains from future triage; Settings lists domain rules and lets you add or remove them.
- **Follow-ups.** Every next step is a follow-up on a person, company or deal: what is owed, whose move it is, when to look at it again, and an optional draft of the message that moves it on. Follow-ups opens on the open ones, earliest first, and is the one place their drafts are shown and edited. Send replies to an email draft in its conversation from the teammate's mailbox that holds it, unless they turned that off in Connections; Approve marks a LinkedIn draft ready for the system that sends it, which claims it once by setting it to Sending and then marks it Sent. Send/Approve opens the same confirmation in the website and embedded Jaz app, saves the latest edit and releases only the reviewed draft and recipients; editing a draft withdraws its approval. The app-only `save_draft` and `send_draft` operations use the connected account; `send_draft` requires `confirmed: true`. This is UI confirmation, not separate human authentication: a holder of the connected account credential can call these operations directly. With `LLM_API_KEY` set, an agent reads each new conversation, in or out, keeps its follow-ups current and drafts replies where one is useful. It also keeps each person's Context, a bullet-point TLDR of the relationship, by merging the current one with what each conversation adds; any writer replaces Context whole, and its history keeps every version. Opening a follow-up shows its conversation, the person's Context and the draft beside the queue, or in its place when the queue is narrow. Sending needs Gmail send access, so accounts connected earlier reconnect once.
- **Privacy.** Metadata is stored for every message, but bodies are fetched only for conversations linked to a kept person or a record, and forgotten when they no longer are. Skipping a thread removes it for good.
- **Sync.** One Temporal workflow per connection backfills mail and calendar (`BACKFILL_DAYS`), then follows Gmail history and Calendar sync tokens every minute, or at once on a push. Meet transcripts are fetched after meetings end and kept, since Meet deletes them after 30 days.
- **Web app.** Triage, conversations, every object as a table, and a page per record with its fields, timeline and a form to log a call or note; ⌘K searches records. In Jaz the same app opens from the sidebar as an MCP App through the `show_crm` tool.
- **Calls and recorders.** Log a call or note in the app or through `log_interaction`. Recorders post conversations to `POST /webhooks/interactions` with an API key; an `external_id` makes a repeated post update one interaction.

## Authentication

- **People** sign in with OpenID Connect. Anyone with a verified email may sign up unless `ALLOWED_EMAIL_DOMAINS` or `ALLOWED_EMAILS` restrict it. Each new person gets a workspace; invites are the only way into another. Every member sees the whole workspace; admins invite people and edit the workspace.
- **Agents** use OAuth 2.1 with dynamic client registration and PKCE, discovered from the 401 on `/mcp`. Personal API keys work as bearer tokens too. The web app calls the same operations at `POST /api/tools/{tool}` with its session.
- **Tenant isolation**: every query and tool is scoped to one workspace; tests attack another workspace's records and interactions by id.

Configuration is documented in `.env.example`.

## ChatGPT plugin

The existing MCP server and embedded app can be connected to ChatGPT using OAuth. See [setup, packaging and remaining requirements](docs/chatgpt-plugin.md); `plugin/` contains the portable manifest and ZIP packager.

## MCP

| Tool | Does |
| --- | --- |
| `list_objects`, `create_object`, `create_attribute` | the schema |
| `add_attribute_option` | reusable select choices, including company categories |
| `edit_pipeline_stage` | rename, reorder or delete stages, preserving records and history |
| `search_records`, `get_record`, `upsert_record`, `delete_record` | records; `get_record` includes how often and when last they were in touch |
| `list_interactions`, `get_interaction`, `search_interactions` | timelines, full conversations, full-text search |
| `log_interaction`, `link_interaction`, `unlink_interaction`, `skip_interaction` | notes, messages and calls/meetings, links to records, removal |
| `list_triage`, `decide_triage` | who is kept, skipped or waiting, and decisions by address or domain |
| `get_triage_settings`, `update_triage_settings` | workspace auto-approval settings, off by default |
| `list_triage_rules`, `forget_triage_rule` | inspect or remove persistent domain decisions |
| `list_connections`, `disconnect` | synced Google accounts |
| `get_workspace`, `update_workspace`, `invite_member`, `delete_workspace` | the workspace, its description and members; deletion requires an admin, its current ID and its name |
| `show_crm` | opens the web app in hosts that support MCP Apps |

`list_interactions` pages with `cursor`, the last interaction ID from the previous page. Dates retain their original precision; transcript turns keep their supplied order and include a time only when one was provided.

`search_records` returns `records` and a `resource_uri` with the same text, attribute filters and limit. MCP Apps hosts render only compact record rows inline, sized to their content. Selecting a row opens its record in the full CRM through the host's app link. `show_crm` also accepts the URI to reopen the results. The `ui://jaz-crm/o/{object}{?q,where,limit,view}` resource template serves the app at that filtered view:

```text
ui://jaz-crm/o/deals?where=%7B%22stage%22%3A%22Lead%22%7D&view=table
```

This URL shows only Lead deals. `q` searches text, `where` is a URL-encoded JSON object mapping attribute slugs to matching values, `limit` is at most 100, and `view=table` selects a table instead of a pipeline. Reference filters accept a record ID or its unique value, such as a company's domain. Every request uses the connected workspace's permissions.

## Develop

```sh
docker compose up -d postgres temporal
cd backend
go build ./... && go vet ./... && go test ./...
sqlc generate                      # after changing SQL in internal/storage/postgres/queries
go run ./cmd/server & go run ./cmd/worker
cd ../frontend && bun install && bun run dev
```
