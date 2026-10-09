# Remove the Notes property

- [x] Explain Notes (property) vs Context vs timeline notes, with production counts.
- [x] Review every CAS Notes value (765 people, 39 companies) and keep only information not held elsewhere.
- [x] Archive Notes on people and companies; drop it from the built-in schema.

## Review outcome

| Notes content | Count | Outcome |
| --- | --- | --- |
| "Contact found via Exa Websets search" and other pipeline boilerplate | 499 | Dropped |
| LinkedIn bio copies on records that link the profile | 69 | Dropped |
| Sources already in Links | 28 | Dropped |
| Research provenance (local file paths, "LinkedIn read") | 26 | Dropped; Context holds the facts |
| Citations: news, lists, data aggregators | 129 | Dropped; the research tables keep them |
| Pages about the person or company (bio, Wikipedia, own site) | 51 links on 42 records | Added to Links |
| Human-written notes | 6 | Merged into Context |

Values stay on the archived property; Settings → Schema → Restore brings them back.

## Verification

- Migration test on Postgres: Notes archived, agent writes rejected, restore returns the values; fails without the migration.
- Full Go suite and frontend check pass. Scratch copy of real data: People Details without Notes; Schema lists Notes under archived properties.
