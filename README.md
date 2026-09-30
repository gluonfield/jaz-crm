# Jaz CRM

A self-hosted CRM that agents operate over MCP. One Go server, backed by Postgres, holds people, companies and other objects whose values keep their history and the source that set them. `SCOPE.md` has the full plan: Gmail, Calendar and Meet sync come next.

## Run it

```sh
docker compose up
```

This starts Postgres (host port 55532) and the server on http://localhost:7500. To connect an agent without signing in, set `OWNER_EMAIL` and `OWNER_API_KEY` in `.env` (copy `.env.example`) and give the agent the key as its bearer token:

```sh
claude mcp add --transport http jaz-crm http://localhost:7500/mcp --header "Authorization: Bearer $OWNER_API_KEY"
```

`docker compose exec server /app/server apikey <email>` mints a key for an existing account.

## Data model

- **Objects** are record types. Every workspace starts with `people` and `companies`.
- **Attributes** are typed: `text`, `email`, `domain`, `phone`, `reference`. Emails, domains and phone numbers are unique within a workspace, so an upsert with a known email updates that person rather than creating a duplicate.
- **Values** are append-only. A change closes the current value and inserts its successor, so every attribute keeps its history.
- **Sources** rank `user` > `agent` > `sync`. A write replaces or removes only values from its own or a lower-ranked source; the rest come back as skipped.

## Authentication

- **People** sign in with OpenID Connect (Google or any other provider). Anyone with a verified email may sign up unless `ALLOWED_EMAIL_DOMAINS` or `ALLOWED_EMAILS` restrict it. Each new person gets a workspace of their own and joins others only by invite.
- **Agents** use OAuth 2.1 with dynamic client registration and PKCE: MCP clients discover the server from the 401 on `/mcp` and walk you through sign-in and consent. Personal API keys work as bearer tokens too.
- **Tenant isolation**: every query and tool is scoped to one workspace; tests attack another workspace's records by id.

| Variable | Purpose |
| --- | --- |
| `PUBLIC_URL` | Base URL; source of the OIDC redirect URI (`PUBLIC_URL/auth/callback`), OAuth issuer and cookie domain |
| `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` | OpenID Connect provider |
| `ALLOWED_EMAIL_DOMAINS`, `ALLOWED_EMAILS` | Optional sign-in restriction |
| `OWNER_EMAIL`, `OWNER_API_KEY` | Optional account and API key provisioned at startup |
| `DATABASE_URL`, `ADDR`, `LOG_LEVEL` | Server basics |

## MCP

| Tool | Does |
| --- | --- |
| `list_objects` | objects and their attributes |
| `search_records` | an object's records by text and attribute values |
| `get_record` | one record's current values |
| `upsert_record` | create or update a record, matched by id or unique values |
| `list_members`, `invite_member`, `rename_workspace` | workspace members; admins invite by email and rename the workspace |

## Develop

```sh
docker compose up -d postgres
cd backend
go build ./... && go vet ./... && go test ./...
sqlc generate   # after changing SQL in internal/storage/postgres/queries
```
