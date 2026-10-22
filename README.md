# Jaz CRM

A CRM that fills itself from Gmail, Google Calendar and Meet transcripts and that agents operate over MCP. People, companies, deals and any objects you add keep every value's history and who set it, ranked person > agent > sync so sync never overwrites an agent and neither overwrites a person. Each record's page shows the emails, meetings, calls and notes with it.

Use it two ways:

- **Standalone app**: server, sync worker and web app, with its own sign-in.
- **Go library**: embedded in another Go program that already signs people in and has a Postgres database.

## Standalone app

```sh
cp .env.example .env   # sign-in, Google sync and optional AI settings are documented there
docker compose up
```

This starts Postgres (host port 55532), Temporal (7533), the server on http://localhost:7500 and the sync worker. Sign in, open Connections and connect a Google account.

CI publishes `ghcr.io/gluonfield/jaz-crm-server` and `ghcr.io/gluonfield/jaz-crm-worker` for every commit on main (`latest` and the commit SHA). Run both from the same tag with the same `DATABASE_URL`, `TEMPORAL_*`, `PUBLIC_URL` and `ENCRYPTION_KEY`; migrations run at startup and `GET /healthz` checks the server.

- **Sign-in**: OpenID Connect or Firebase (`AUTH_PROVIDER`). New people get a workspace of their own; invites are the only way into another.
- **Agents**: connect to `/mcp` with OAuth 2.1 (dynamic client registration, PKCE) or an API key as a bearer token, and list the tools from there. The web app calls the same operations at `POST /api/tools/{tool}`.

```sh
claude mcp add --transport http jaz-crm http://localhost:7500/mcp --header "Authorization: Bearer $OWNER_API_KEY"
```

ChatGPT setup and packaging: [docs/chatgpt-plugin.md](docs/chatgpt-plugin.md) and `plugin/`.

## Go library

```sh
go get github.com/gluonfield/jaz-crm/backend@<commit>
```

```go
import (
	"github.com/gluonfield/jaz-crm/backend/crm"
	"github.com/gluonfield/jaz-crm/backend/crm/records"
)

// When the host deploys, on the pool it already has:
err := crm.Migrate(ctx, pool, "crm")

// Once, in the host's composition root:
c, err := crm.Open(ctx, pool, crm.Options{Schema: "crm"})

// When the host creates a team; it keeps owner.WorkspaceID on the team:
owner, err := c.CreateWorkspace(ctx, team.Name, crm.Member{
	Principal: crm.Principal{Issuer: "myapp", Subject: user.ID},
	Name:      user.Name,
	Email:     user.Email,
})

// Per request, after the host has signed the person in and checked the team:
actor, err := c.ActAs(ctx, crm.Principal{Issuer: "myapp", Subject: user.ID}, team.CRMWorkspaceID)
company, skipped, err := c.Records.Upsert(ctx, actor, actor.Source(), records.Write{
	Object: "companies",
	Set:    map[string][]string{"name": {"Acme"}, "domains": {"acme.com"}},
})
```

- **The host** authenticates people, decides who belongs to which team, owns the database and the pool, and chooses when `Migrate` runs. It calls `AddMember` when someone joins a team and stops calling `ActAs` for them when they leave.
- **The CRM** owns its schema: tables, migration history, records with their history, and one member per person per workspace, linked to the host's `Principal` (an issuer naming the host and the host's stable user ID). It never reads a host user ID as a member ID or matches people by email. Every call is scoped to the workspace of its `records.Actor`; set `actor.Agent` when software acts for the person.
- **No side effects at construction.** `Open` checks that the schema is migrated and changes nothing; neither function starts servers, jobs or provider clients or closes the pool. `Migrate` connects through a short-lived pool built from the host pool's configuration. Each statement sets `search_path` inside its own transaction, so the host's connections keep their settings. `Migrate` needs permission to create the schema and the `pg_trgm` extension; install `pg_trgm` beforehand to keep it in `public`.
- **Jobs stay standalone.** Gmail and Calendar sync, AI triage and follow-up drafting run in the standalone worker (Temporal, Google, OpenAI). An embedded CRM gets them only by running that worker, which is not covered yet.
- **Interface.** The host renders CRM data in its own UI from `c.Records` and `c.Interactions`; the web app and MCP server are the standalone app's. Writes notify the Postgres channel `changes` with the workspace ID, for hosts that refresh open pages.
- **Other CRMs.** Connectors are not built. One would write through these services as `records.SourceSync`, matching records by unique attributes such as emails and domains, so people's and agents' edits always win.

The Go API follows commits on main; there are no tagged releases yet. [`examples/embed`](examples/embed) is a separate module that runs this flow on a scratch database next to host tables.

## Develop

```sh
docker compose up -d postgres temporal
cd frontend && bun install && bun run build   # also generates the MCP App the server embeds
cd ../backend && go build ./... && go vet ./... && go test ./...
go run ./cmd/server & go run ./cmd/worker
cd ../frontend && bun run dev
```

`bun run check` in `frontend` runs unit tests, type checks, lint, builds and headless browser scenarios. Run `go test ./...` in `examples/embed` after changing the public API, and `sqlc generate` in `backend` after changing SQL in `internal/storage/postgres/queries`. [CLAUDE.md](CLAUDE.md) and [AGENTS.md](AGENTS.md) hold the engineering and design rules.

## Licence

[MIT](LICENSE)
