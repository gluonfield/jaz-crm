# Companies request batching

- [x] Fetch the Companies table and linked People in one request, with one batched relationship read rather than one query per row.
- [x] Preserve photos, person links, three visible people and the More link.
- [x] Verify deployed CRM and Tasks response compression: public HTML and JavaScript negotiate Brotli/gzip; CRM JSON and Tasks GraphQL negotiate Zstandard in Chromium.
- [ ] Review, run full checks, commit/push and verify the deployed page's request count.

Observed before the fix: 24 companies triggered 25 `search_records` requests. Tasks uses GraphQL; CRM uses its shared HTTP/MCP tools. Extend the existing CRM search contract with optional related-record selection so both transports return the same result.

Full Go build/vet/tests and frontend typecheck/lint/web/MCP builds pass. Real PostgreSQL/MCP coverage checks ordering, photos, empty companies, email labels, reassignment, filtered parents and workspace isolation. The batch regression requires one relationship query for 25 companies; a compiling negative control that queried each parent separately failed with 25 queries. Review corrected missing-name labels and removed a redundant slice copy. No new dependency or stored reverse relationship.
