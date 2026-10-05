# Shortuuid IDs

- [x] Generate new product IDs with `github.com/lithammer/shortuuid/v4` and its default alphabet, retaining all 128 UUID bits.
- [x] Store product IDs and references as PostgreSQL text; preserve existing ID strings and links.
- [x] Support existing UUID IDs and new shortuuids in lookups and relationships.
- [x] Keep provider identifiers and authentication secrets in their existing formats.
- [x] Verify populated upgrades, mixed references and case-sensitive lookups against isolated PostgreSQL databases.
- [x] Complete code review, Go build/vet/full tests and frontend checks.
- [x] Confirm the upgrade test fails when new-record generation is reverted to UUID strings.
- [x] Commit and push.
