# Authentication

Tasks and CRM use the same versioned Go sign-in module:
https://github.com/gluonfield/jaz-tasks/tree/main/auth

`AUTH_PROVIDER=oidc` is the default. Configure `OIDC_ISSUER`,
`OIDC_CLIENT_ID` and `OIDC_CLIENT_SECRET`, with
`PUBLIC_URL/auth/callback` registered at the identity provider.

For Firebase, set `AUTH_PROVIDER=firebase` and `FIREBASE_CONFIG` to the
Firebase web configuration JSON containing `projectId`, `apiKey`,
`authDomain` and `appId`. Enable Google sign-in in Firebase and add the
app's hostname to Authentication's authorised domains. No service-account
key is required. Use the same Firebase project for the shared hosted user pool.

`/login` serves the shared sign-in page. `/auth/config` exposes the active
provider and the public Firebase web configuration. It exposes no OIDC secrets.

The app keeps its workspace memberships, session cookies and MCP OAuth grants.
Verified Firebase Google subjects link to existing Google memberships; matching
an arbitrary account's email does not merge its data. Firebase tokens from other
projects, tenants or stale sign-ins are rejected. The current browser method is
Google sign-in in the Firebase project's default pool.

Existing app sessions keep their normal expiry and revocation policy. Disabling
an account in Firebase does not automatically revoke already-issued app sessions.

Verification: run the shared module's `go test -race ./...` and `go vet ./...`,
then each backend's `go test ./...`, `go vet ./...` and `go build ./...` with its
Postgres test database available. Run each frontend's `bun run check`.


## Hosted configuration and acceptance

Railway uses `AUTH_PROVIDER=firebase` with the `jaz-production-72cc9` project
for Tasks, CRM Server and CRM Worker. Both domains are authorised in Firebase;
Google browser sign-in completes on both apps. OIDC settings remain available
for an environment selecting `AUTH_PROVIDER=oidc`.

Shared module: `github.com/gluonfield/jaz-tasks/auth@v0.1.1`.
New projects can mount the same handler and supply their account/session callback.

Verified 2026-10-01: full backend test/build/vet, frontend checks, shared-module
race tests/CI, Postgres membership migration, and negative controls for signature
verification and identity linking. Production health/config/login checks and
existing authenticated MCP initialize/profile reads pass. CRM image publication
passed GitHub run 36855494062; deployed images use integration commit `67ad4a5`.
Tasks integration is `01c44e4`. Live browser sign-in and existing provisioned
owner MCP credentials both pass acceptance checks.

## Shared sign-in interface

| Before | After |
| --- | --- |
| Separate Tasks React login route and CRM server login page | One shared server page with each app's name and provider button |
| Sign-in control handled by each app | Shared Google mark, 42px control, keyboard focus and light/dark colours |
| Provider flow duplicated in each app | Shared loading, cancellation, failure and retry behaviour; Firebase waits for SDK readiness |
