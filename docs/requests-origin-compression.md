# Origin response compression

- [x] Compress CRM responses in Go before they leave Railway, preserving HTTP negotiation and streaming MCP.
- [x] Reuse Tasks' first-party standard-library gzip middleware.
- [x] Verify real MCP app downloads through the native Go client and on the wire.
- [x] Run backend/frontend checks and code-quality review.
- [x] Commit/push and verify production: `1f19a6b` succeeded on both Server and Worker, using shared middleware `httpx/v0.1.1`.

The real MCP app response shrank from 1,943,144 to 683,801 bytes (64.8%). Compression changes outbound bytes, not request frequency or RAM usage. Brotli would require a separately approved dependency.

Authenticated production MCP verification: 1,942,611 bytes with identity, 683,618 with gzip (64.8% smaller), identical decoded app. Before deployment, both encodings returned 1,942,611 bytes.
