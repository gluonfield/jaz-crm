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
