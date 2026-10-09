# Profile Links

Augustinas questioned the per-network `linkedin_url` and `x_url` fields on 2026-10-09 and asked whether a profile should simply have links, general but practical. A strict review of the interaction-channel work found the per-network fields were the source of its remaining special cases.

- [x] People and Companies have one Links field: many links, any site, recognised by host for icon and name.
- [x] Links identify records like email addresses: a link is compared by host and path, so `upsert_record` with a link finds the record holding it.
- [x] LinkedIn values move into Links in place, keeping their history; X values move too and the X field goes.
- [x] A chat participant is a link handle that joins the person whose Links hold it, with no per-network matching.
- [x] Email addresses, phone numbers and a company's Website stay separate fields.

## Findings

- Production on 2026-10-09: 863 People LinkedIn values in CAS, all `linkedin.com/in/…`, none colliding once compared by host and path; 29 Companies LinkedIn values, none colliding; no X values; no saved filter uses either field; only email handles exist.

## Behaviour

- `records.LinkKey` is the one rule for comparing links; unique URL values use it, and chat participants are link handles whose value is that key, so a participant joins the record holding the link through the same match as an email address.
- Handle kinds are email, phone and link. `@name` becomes `x.com/name` or `t.me/name` on X and Telegram.
- Moving values in the migration leaves each record's last update unchanged.

## Verification

- Full Go suite, vet and gofmt pass; frontend tests, typecheck, lint and both builds pass.
- Tests cover link variants finding one person without duplicating the link, a link held by another person being refused, and a migration from version 50: fields renamed in place with history, X values moved, the X field removed, saved filters rewritten, handle kinds collapsed, links found by variant, handles paired with their owners, and no record's last update changed.
- Negative controls fail without the key rewrite, without moving X values, with plain lower-case URL keys, and without standing down the last-update trigger.
- Rehearsal on a restored production copy: version 51; 863 People links plus 4 history rows and 29 Companies links under Links with host-and-path keys and none empty; no record's last update changed; Jim Mayer's page shows Links and his follow-up profile shows LinkedIn with its mark.

## Rollout

- `358eed6` pushed to main; Railway Server and Worker report SUCCESS for it.
- Read-only production checks: migration version 51; People and Companies have Links, multi-valued and identifying, holding 863 and 29 links with no empty keys; no record's last update moved; the live API returns Jim Mayer's LinkedIn under `links`. The production backup taken before the push was deleted after these checks.
