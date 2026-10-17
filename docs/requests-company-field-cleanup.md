# Company Field Cleanup

- [x] Replace CAS's manually stored Last Activity Date with calculated Last Contact in company and people tables.
- [x] Preserve Outreach Notes as company timeline notes.
- [x] Merge Primary Activities and Key Products & Services into Description, preserving existing text and citations.
- [x] Add Manufacturer to Categories for companies marked Yes; archive the separate property.
- [x] Archive Fit Profile without replacing it.
- [x] Rehearse the CAS-only migration, preserve property history and record update timestamps, and verify other workspaces remain unchanged.
- [x] Run the complete backend and frontend checks and verify the real screens.
- [ ] Commit, push and verify the production rollout.

## Verification

Migration 55 targets only the CAS workspace ID. Archived properties retain their
values and history. Other workspaces retain their custom schemas.

Replayed 13,370 current values from 2,816 CAS companies locally: 1,847 existing
descriptions were expanded, all 6,414 nonblank source description/activity/product
values remain represented, all prior categories remain, and 151 substantive
outreach notes retain their text and original stored date. Pure Exa search-count
bookkeeping stays in the archived property. Manufacturer categories total 1,855,
including pre-existing tags; all 1,837 Yes values are represented. No record's
Last Updated changed.

The production-Postgres migration test covers preservation, deduplication, archive
restore, rejected agent writes to retired fields, and workspace isolation. A
no-op migration control fails on a retired property still appearing. Full Go
build/vet/tests and frontend tests/typecheck/lint/both builds pass. The real app
shows calculated contact dates in company and people tables, separately from
record edits, with light/dark and 390px checks.

Scratch snapshots, replay and checks: runs/company-field-cleanup/.
