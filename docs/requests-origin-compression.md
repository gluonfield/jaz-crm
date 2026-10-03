# Origin response compression

- [x] Compress CRM responses in Go before they leave Railway, preserving HTTP negotiation and streaming MCP.
- [x] Reuse Tasks' first-party standard-library gzip middleware.
- [x] Verify real MCP app downloads through the native Go client and on the wire.
- [x] Run backend/frontend checks and code-quality review.
- [ ] Commit/push and verify production (deployment outcome recorded in session memory).

The real MCP app response shrank from 1,943,144 to 683,801 bytes (64.8%). Compression changes outbound bytes, not request frequency or RAM usage. Brotli would require a separately approved dependency.
