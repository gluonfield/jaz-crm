# Shared authentication request ledger

- [x] Consume the same sign-in module as Tasks.
- [x] Support Firebase and configurable direct OIDC.
- [x] Preserve existing Google memberships during migration.
- [x] Verify authentication, workspace policy and MCP OAuth.
- [x] Commit and push verified changes.
- [x] Deploy Railway Server/Worker with the supplied shared Firebase pool.
- [x] Verify production health, configured pool and browser sign-in.

Requested 2026-10-01. Details and validation are recorded in authentication.md.

Verified: full Go suites, build/vet, frontend checks, signed-token HTTP exchange,
CSRF/origin rejection and real Postgres membership migration. Negative controls
fail when signature verification or identity migration is disabled.
Both repositories are pushed and Railway deployments report SUCCESS.
Both apps expose the supplied Firebase pool; real Google browser sign-in and
authenticated MCP initialize/profile reads pass (14 Tasks tools, 30 CRM tools).
