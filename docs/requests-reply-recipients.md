# Reply recipients

- [x] Keep teammates in reply-all To/Cc; exclude only the sending mailbox and its aliases, respect Reply-To and deduplicate addresses.
- [x] Repair existing drafts that match the old automatic defaults, preserve custom lists, and send exactly the recipients shown for review.
- [x] Show every reply recipient without truncation.
- [x] Review, run the full verification path and browser checks.

Verification: Go 1.26 build, vet and full suite passed against isolated PostgreSQL. Regression coverage exercises teammate To/Cc, Reply-To, duplicate addresses, sender aliases, saved legacy and custom lists, sender changes and exact outgoing MIME headers. Browser checks use the production build with captured HTTP response shapes in both themes at 1280px and 620px, verifying wrapping, refreshed previews and matching send payloads. Strict code review completed; no new dependency.

The final suite also passed after rebasing onto the concurrent drafting-context change. A temporary negative control that drops a teammate fails five recipient scenarios, confirming the regression checks exercise the bug. Delivery is recorded in session memory.
