# Engineering Rules

- Never add useless or fake tests. Every test must run in the normal verification path, exercise real production behavior, and fail when that behavior breaks. Delete tautologies, tests of trivial helpers or forwarding, checks of source text or internal constants, duplicate coverage, and permanently skipped or inactive tests.
- Fakes may isolate external boundaries; they must preserve the real contract and never stand in for end-to-end verification.
