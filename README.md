# Jaz CRM

A CRM that fills itself from Gmail, Google Calendar and Meet transcripts and that agents operate over MCP. People, companies, deals and any objects you add keep every value's history and who set it, ranked person > agent > sync so sync never overwrites an agent and neither overwrites a person.

Embed it in a Go program that already signs people in and has a Postgres database, or run it as a standalone app with its own sign-in.

## Go library

```sh
go get github.com/gluonfield/jaz-crm@<commit>
```

```go
import (
	crm "github.com/gluonfield/jaz-crm"
	"github.com/gluonfield/jaz-crm/records"
)

// On deploy: create or upgrade the CRM's own schema in your database.
err := crm.Migrate(ctx, pool, "crm")

// Once per process, on the pool your program already has.
c, err := crm.Open(ctx, pool, crm.Options{Schema: "crm"})

// Per request, after your program has signed user in and checked that they belong to org.
actor, err := c.Actor(ctx, crm.User{Tenant: org.ID, ID: user.ID, Name: user.Name, Email: user.Email})
acme, skipped, err := c.Records.Upsert(ctx, actor, actor.Source(), records.Write{
	Object: "companies",
	Set:    map[string][]string{"name": {"Acme"}, "domains": {"acme.com"}},
})
history, err := c.Records.History(ctx, actor, acme.ID)
```

- **Your program** signs people in, decides which tenant (company, account) each belongs to and checks it on every request, owns the database and the pool, and runs `Migrate` when it deploys. Build `crm.User` from your own records, never from request input. Removing someone from a tenant in your program removes their access.
- **The CRM** scopes every call to the actor's tenant: everyone in a tenant shares its records and no other tenant sees them. The tenant ID becomes the actor's `WorkspaceID`; a tenant's first request creates its workspace with the standard objects (people, companies, deals, follow-ups). The CRM keeps a member for each user in each tenant, found by your user ID and never by email and updated when the name or email changes, so history names who wrote what. Set `actor.Agent = true` when software acts for the user.
- **Nothing starts.** `Open` checks that the schema is migrated and changes nothing, and neither function closes your pool. Each statement sets `search_path` inside its own transaction, so your connections keep their settings. `Migrate` connects through a short-lived pool built from yours and needs permission to create the schema and the `pg_trgm` extension; install `pg_trgm` beforehand to keep it in `public`.
- **Standalone only:** Gmail and Calendar sync, AI triage and follow-up drafting run in the standalone worker (Temporal, Google, OpenAI), and the web app and MCP server are the standalone app's. Show CRM data in your own UI from `c.Records` and `c.Interactions`; writes notify the Postgres channel `changes` with the tenant ID.
- **Other CRMs:** connectors are not built. One would write through these services as `records.SourceSync`, matching records by unique attributes such as emails and domains, so people's and agents' edits win.

The API follows commits on main; there are no tagged releases. [`examples/embed`](examples/embed) runs this flow beside a host's own companies, users and memberships in one database.

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

Public OAuth client registration accepts a 32 KiB body, a 256-byte name and up to eight redirect URIs of 2,048 bytes each. Each server allows a burst of ten registrations, replenishing one every ten seconds; rate-limited clients receive `429` and `Retry-After`.

Only a mailbox’s owner can disconnect it. Email images stay hidden until the viewer chooses **Load images**, and a new message requires new approval.

ChatGPT setup and packaging: [docs/chatgpt-plugin.md](docs/chatgpt-plugin.md) and `plugin/`.

## Develop

```sh
docker compose up -d postgres temporal
cd frontend && bun install && bun run build   # also generates the MCP App the server embeds
cd .. && go build ./... && go vet ./... && go test ./...
go run ./cmd/server & go run ./cmd/worker
cd frontend && bun run dev
```

`bun run check` in `frontend` runs unit tests, type checks, lint, builds and headless browser scenarios. Run `go test ./...` in `examples/embed` after changing the public API, and `sqlc generate` after changing SQL in `internal/storage/postgres/queries`. [CLAUDE.md](CLAUDE.md) and [AGENTS.md](AGENTS.md) hold the engineering and design rules.

## Licence

[MIT](LICENSE)
