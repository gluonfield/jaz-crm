# Engineering Rules

- Use Go 1.26.
- Keep code and JSON minimal. Each line of code should fight for its existence; every field and line must earn its place.
- Prefer implementations that reduce total code over ones that add more. Adding lines is a cost to justify; a good fix often deletes code, collapses branches, or moves an invariant to the layer that already owns it.
- Write one statement per line; never join statements with semicolons.
- Trim strings at real input boundaries only: user input, config files, env vars, HTTP payloads, CLI args, and persisted loose text. Do not sprinkle `strings.TrimSpace` over internal constants, typed IDs, enum values, or values that have already crossed a validation boundary.
- When there's an opportunity for dramatic simplification or restructuring, bring it up. Favor moves that delete layers, unify shapes, collapse special cases, or make the design inevitable over incremental patches.
- Bug fixes should first look for deletion or correction of the underlying contract. A solution that only adds branches, flags, helpers, or UI glue is suspicious.
- Do not add code comments until they are genuinely needed to explain specific behavior the code itself cannot describe.
- Keep concrete implementations focused and interfaces small.
- Put behavior in the layer that owns the concept.
- Split files when a feature starts mixing transport, persistence, formatting, and UI concerns. Avoid pushing files toward 1k lines without a strong structural reason.
- Keep feature diffs scoped. Do not mix unrelated UI polish, dependency churn, or generated output into behavioral changes.
- Keep `main.go` files as command dispatch and process entrypoints only. Domain types, helpers, clients and request/response shapes belong in the package that owns that concept.
- Use Fx constructors directly in `fx.Provide`; avoid pass-through wrappers.
- Do not add defensive nil checks for required constructor-injected dependencies. If a required Fx service is missing, fail fast; model truly optional dependencies explicitly.
- Never commit secrets. Configuration comes from flags and env vars; document them in `.env.example`.
- Target deployments run the server on a VM and clients on user computers; never assume client-local file paths are visible to the server.
- Before handing off a completed feature or fix, run a code-review pass.
- Every test you add must be useful: it must run in the relevant verification path and either protect real behavior or clarify a tricky contract. Tests that need Postgres fail (not skip) when it is unavailable.

## Backend architecture

- Imports point one way: `server` -> `httpapi/<feature>` -> `internal/<feature>` -> `storage` contracts. Feature packages must not import `server`, `httpapi`, or `storage/postgres`; the Postgres adapter must not import feature packages.
- `internal/server` is the HTTP process shell: routing, CORS and health. Feature behavior does not live there.
- `internal/records` owns the CRM data model: objects, typed attributes, records and their values. Values are append-only: a change closes the current row (`active_until`) and inserts its successor, so never update or delete a value in place. Every value records its source (`user` > `agent` > `sync`); a write replaces or removes only values from its own or a lower-ranked source and reports the rest as skipped.
- Unique attributes (emails, domains, phone numbers) identify records within a workspace through `record_values.unique_key` and its partial unique index; an upsert matches on them instead of creating duplicates.
- Add an attribute type by extending the `attributes.type` check in a new migration and `records.normalize`; add value columns only with the type that needs them.
- Transports (the MCP endpoint, future adapters) translate protocol shapes only.
- Services take the actor explicitly; services never read HTTP headers.
- Keep multi-record writes atomic in storage methods. `WriteRecord` locks the record and runs the service's mutation inside the transaction.

## Authentication

- `internal/auth` owns identity. People sign in with OIDC and hold a server-side session cookie; agents and apps get OAuth 2.1 tokens from our own authorization server; personal API keys are a secondary path for scripts. All three resolve to the same `auth.Actor`.
- Tokens, keys, codes and session ids are opaque random values; store only their SHA-256. Never log them.
- `PUBLIC_URL` is the single source for the OIDC redirect URI, the OAuth issuer, metadata URLs and the cookie domain. Do not add parallel settings.
- Every 401 from `/mcp` carries `WWW-Authenticate: Bearer resource_metadata=...`; keep it on the go-sdk bearer middleware.
- Settings endpoints that mint or revoke credentials require a browser session, never a bearer token.
- `internal/workspaces` owns membership: a person (OIDC identity) has one user row per workspace; new people get their own workspace with the standard objects; invites are the only way into another. A session or token acts as exactly one user row, so as exactly one workspace. Every member sees every record; the only role is `admin`, who invites people.
- Tenant isolation is a hard requirement. Every storage query touching workspace data filters by `workspace_id` or reaches it through an already-scoped record. Extend `TestTenantIsolation` (MCP) and `TestReferencesAndSearch` (records) whenever you add a query or tool that takes a reference.

## Postgres and sqlc

- Migrations live in `backend/internal/storage/postgres/migrations` (goose, embedded, run at startup). Never edit a migration that has shipped; add a new one.
- Query SQL lives in `backend/internal/storage/postgres/queries/<feature>`; sqlc output goes to `backend/internal/storage/postgres/generated/<feature>` and is checked in. Handwritten `storage/postgres/*.go` files may open the pool, run migrations, manage transactions and map rows, but must not contain query SQL.
- `storage` records and inputs mirror the sqlc models and params field for field, so the adapter converts with a type conversion. A schema change fails to compile until the record follows; keep it that way instead of writing field-by-field mappers.
- Regenerate with `sqlc generate` in `backend`. sqlc cannot parse multi-array `unnest`; pair arrays with `generate_subscripts`.

## Testing

- Test at the lowest boundary that protects behavior. Records tests run against a throwaway Postgres database (`postgrestest.New`); MCP and auth tests drive the real HTTP handlers. Start Postgres with `docker compose up -d postgres`.
- Verification before every push: `go build ./... && go vet ./... && go test ./...` in `backend`.

## Scope

- `SCOPE.md` is the product scope and milestone plan. Update it when a decision changes.
