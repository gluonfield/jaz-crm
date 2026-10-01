# Companies People column

- [x] Show linked people in the Companies table.
- [x] Use profile photos when available, with the existing initials fallback.
- [x] Keep cells compact; open each person from their link and additional contacts from More.
- [x] Verify the real page in both themes and at a narrow width.
- [x] Review, run project checks, commit/push and verify activation.

Frontend typecheck, lint and both app builds pass. Backend build, vet and the full test suite pass. Review reuses the existing relationship query and avatar renderer; no stored reverse relationship or new dependency.

`e456ea5` is pushed and deployed locally and on Railway. Real CAS relationships and person navigation pass. Both themes preserve 40px rows and 18px avatars; a 760px iframe keeps overflow inside the table. A temporary browser response fixture verifies three people plus More, a loaded photo, failed-photo initials and More opening the real company-filtered People page. No CRM records were changed. Screenshot capture returned another Jaz window, so pixel-level visual review remains unverified.
