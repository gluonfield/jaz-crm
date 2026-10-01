# Shared authentication request ledger

- [x] Consume the same sign-in module as Tasks.
- [x] Support Firebase and configurable direct OIDC.
- [x] Preserve existing Google memberships during migration.
- [x] Verify authentication, workspace policy and MCP OAuth.
- [ ] Commit and push verified changes.
- [ ] Deploy Railway Server/Worker with the supplied shared Firebase pool.
- [ ] Verify production health, configured pool and browser sign-in.

Requested 2026-10-01. Details and validation are recorded in authentication.md.

Verified: full Go suites, build/vet, frontend checks, signed-token HTTP exchange,
CSRF/origin rejection and real Postgres membership migration. Negative controls
fail when signature verification or identity migration is disabled.
Publication/deployment is underway; live browser sign-in remains pending.
