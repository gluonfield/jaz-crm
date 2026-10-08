# Core CRM Properties Review

- [x] Audit canonical schema, existing workspace lifecycle and property protection against the built-in core-field requirement.
- [x] Supply 13 common People/Companies profile, ownership and note fields, plus deal expected close date, without custom-schema creation.
- [x] Migrate existing workspaces atomically; adopt compatible property identities/options/values/history and preserve unrelated custom fields.
- [x] Verify fresh workspace writes, migrated data, property protection and conflict rollback/recovery through production PostgreSQL/storage/HTTP.
- [x] Run full checks and strict review.
- [x] Commit/push and verify deployment and production schema/UI read-only.

## Confirmed Findings

1. Core profile/note properties were still workspace extensions; fresh workspaces lacked them.
2. Updating StandardObjects alone cannot upgrade existing workspaces. Historical migrations must retain frozen definitions, and adoption must preserve stored property identities.
3. Static object definitions mixed with value parsing. Move the existing canonical catalog into its own domain file; keep the shared typed storage/history and existing UI.
4. Number validation accepted `NaN` and infinities. Reject them in the existing validator.
5. The header's identity line included short Notes. Exclude narrative fields in the existing display rule, retaining normal field editing.
6. Production and historical test migration registration repeated the same list. Use one registry.

Activity-derived fields and a consistent money/currency contract remain separate proposals, beyond this property-contract review. The proposed Currency property is deferred because current value formatting assumes USD. No communication/outreach data is moved back onto People. New records follow the existing creator-as-owner default; migration leaves existing owners and unassigned records unchanged.

## Verification

- Full Go suite, build and vet pass on the final source revision. Frontend check passes (10 tests, typecheck, lint, production and embedded builds). No new dependencies.
- Real PostgreSQL tests cover fresh writes, safe adoption with exact stored history retained, custom options/names/view retention, protection, and incompatible-definition rollback/recovery. Existing unassigned records stay unassigned.
- Negative controls fail for removed migration coverage and disabled finite-number validation.
- Real HTTP/browser flow starts with the default schema, with no custom-property creation. Notes edit/save/reload, LinkedIn and member controls work; the Notes menu offers Rename without Archive/Delete. Light/dark screenshots inspected. The existing fixed desktop sidebar leaves record content cramped at 390px; responsive navigation is a separate existing issue.
- Read-only CAS preflight: nine existing definitions are compatible; four new properties are required. Existing uniqueness, names and choices are retained.
- Strict review: one canonical catalog, frozen migration snapshot, atomic adoption, shared production/test migration registry, existing field controls. No new runtime layer or file approaching 1,000 lines.

| Before | After |
| --- | --- |
| Short Notes appeared beside job title | Notes stay in Details; identity facts remain in the header |
| Common fields required workspace extensions | 13 additional protected core fields work in new and existing workspaces |

## Rollout

- Code: `39292486a2a363ab6e78583f65ccf433e7b12fa6`, pushed to main. Both Railway Server (`9d8160c1-e522-49f9-9b69-5bd105127247`) and Worker (`0a87a061-aeba-479c-a011-93fb65949c31`) report SUCCESS for this exact commit. CI run `37853791525` verification passed; image publication is separate.
- Read-only production checks: all 13 fields present/protected; every pre-existing CAS definition retains its name/type/options/uniqueness/archive state; health returns 200; Person and Schema pages expose the new fields. Deployed stylesheet hash matches the verified local build. The record route matches byte-for-byte after normalizing only its bootstrap-import filename; deployed Notes exclusion is verified.
- Disposable browser stack stopped successfully and its PostgreSQL cleanup completed. No live record mutations were used for verification.
