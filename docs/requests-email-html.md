# Email HTML rendering

- [x] Diagnose the screenshot's flattened signature link.
- [x] Preserve both MIME body alternatives through fetch and storage.
- [x] Render HTML messages and signatures through one safe renderer.
- [x] Refresh previously fetched emails without clearing their current text.
- [x] Verify real MIME fetch, persistence, display, plain-text fallback and content removal.
- [x] Verify the website and embedded app in both themes and narrow layouts.
- [x] Run full backend/frontend checks and code-quality review.
- [x] Commit, push and verify production.

The original message body retained only its plain-text alternative. Gmail's linked signature label therefore becomes text plus its URL. Keep plain text for search, previews and drafting, and preserve HTML separately for full email display. Reuse the existing signature renderer's explicit element reconstruction; display no injected scripts or event attributes. Previously fetched, linked email bodies refresh through the normal worker because HTML is unknown until fetched; plain-text-only messages record an empty HTML value.

Review: HTML has one parser and one renderer shared with signatures; text/HTML writes are atomic, metadata resync retains fetched HTML, removed provider content clears both alternatives, and plain-text-only fetches finish rather than looping. No dependency added.

Validation: Go build/vet/full tests and frontend typecheck/lint/both builds pass. Built web and opaque-origin embedded app pass 16 checks across Follow-ups/conversation, light/dark and 1280/720px widths; formatting/safety check passes. No real email sent during verification. Evidence: `/tmp/crm-email-html-20261003/results.json` and screenshots.

Production: fix `e8f9495` and scope correction `8631b9f` are pushed. Server deployment `8f31a4b2-6257-4c2a-9a14-471158526e3a` and Worker `a445cfae-7885-44a9-8443-79f5d5128af4` succeeded at descendant `57d4e0d`. The reported sent message now stores its 273-character HTML alternative and the live CAS API returns the labeled LinkedIn anchor. Deployed web/embedded Follow-ups and conversation checks pass using that captured API response, including quote omission and labeled-link routing. Evidence: `/tmp/crm-email-html-20261003/deployed-results.json`.
