# Drafting context

- [x] Keep each person's name, fields and relationship summary together, removing duplicate input contexts.
- [x] Select Company knowledge through a persisted workspace page reference, preserve existing unambiguous Company roots, and include readable descendant paths.
- [x] Verify multiple conversation contacts, exclusion of unrelated employees, root renames, pagination and workspace isolation through the real request path.
- [x] Review and run all required checks before committing and pushing the correction.

- [x] Initial version: one structured LLM call. Superseded by the authorized agentic loop below.
- [x] Supply the full stored conversation rather than its last 24,000 bytes.
- [x] Supply the workspace's Company page and its nested pages from current records.
- [x] Supply linked people, their companies, related deals and open follow-ups with their current data.
- [x] Identify the sender explicitly when the conversation has a known owner.
- [x] Verify the actual LLM request through the production storage and client paths, review, run the full backend suite, commit and push.

Company knowledge starts at the workspace's persisted `company_page_id` and includes every descendant's full content and root-relative path, such as `Company / Strategy`. Admins select it with `update_workspace`; omission preserves it and an empty string clears it. The page must belong to the workspace. Renaming preserves the selection; deleting the root clears it. Migration 0027 selects an existing unambiguous top-level Company page once. Ambiguous roots need explicit selection, and new workspaces have no implicit root.

Each person appears once in `records`, with their name and `values.context`; participants carry the corresponding `person_id` when known. Selection starts from conversation-linked records and expands to their companies, deals and open follow-ups, without pulling in every employee. The output `contexts` array remains the list of summary updates. Conversation context uses the complete stored thread; the person's summary supplies an initial view of past conversations, and the agent can retrieve their full stored history when relevant. Records and documents may contain internal information that should not be copied into external replies.

Verification: Go 1.26 build, vet and the full backend suite passed against an isolated PostgreSQL database. The real Agent → LLM HTTP request test covers messages and documents longer than 24 KB, original quoted content, 103 Company documents across multiple query pages, current contact/company/deal values, sender identity, workspace isolation, and refreshed page content on the next conversation change. Three temporary Go overlays each made the regression test fail as expected: restoring display cleanup, cutting message text, and reading only the first document page. Test databases were removed by their cleanups.

Correction verification: Go 1.26 build, vet and the full backend suite passed. The request test now exercises sync links for two contacts at one company, excludes an unrelated colleague and same-title page, verifies complete document paths after renaming the root, and verifies clearing prevents title-based fallback. MCP tests cover persistence, omission, clearing, deletion, admin permissions, workspace isolation and atomic rejection of invalid roots. Migration tests cover absent, unique and ambiguous roots, including nested same-title pages.

Initial implementation review: context collection stays in followups, original/display text selection stays in interactions, and root selection belongs to workspace settings. Selection validation and workspace updates happen in one SQL write. The traversal uses one path map for both paths and deduplication. Source-read failures release the conversation claim for retry. That revision added no dependency or model tool loop; the agentic extension below supersedes the one-shot design. Oversized provider requests fail instead of silently dropping input; existing saved drafts are regenerated only when their conversation changes.

## Read-only drafting agent (2026-10-03)

- [x] Implement a real tool-calling loop with the official OpenAI Go SDK.
- [x] Expose useful, workspace-scoped read-only CRM tools, including arbitrary page discovery and full prior-conversation reads.
- [x] Preserve complete input and provider reasoning/tool items; use stable prefixes and cache keys for provider-managed KV caching.
- [x] Verify successful retrieval, denied writes, workspace isolation, pagination, error handling, cancellation and loop bounds.
- [x] Review and run the full backend suite. Publication and rollout are recorded in Git/CI and the session ledger.

The existing official `openai-go/v3` Responses client now loops over model-selected function calls. It exposes six reads backed by the same domain services as MCP: `list_objects`, `search_records`, `get_record`, `list_interactions`, `search_interactions`, and `get_interaction`. The actor/workspace is fixed outside model arguments, the advertised handlers form the executable allowlist, and arguments are validated against their strict schemas before execution. No write or send handler is reachable. Record references include IDs for navigation; search results identify records to open in full. Initial context and the existing final-plan write checks are preserved.

Every turn appends all provider output items, including encrypted reasoning, followed by call-ID-linked tool results. Instructions, sorted schema requirements, tool order, model options and the workspace cache key remain stable. Responses use `store:false`, explicitly disable truncation, and retain OpenAI's default automatic prompt caching rather than implementing a local KV cache. Actual provider usage, including cache reads/writes when supplied, is logged per response. OpenAI documents [prompt caching](https://developers.openai.com/api/docs/guides/prompt-caching) and [reasoning replay](https://developers.openai.com/api/docs/guides/reasoning).

A draft has at most 12 model turns, 48 tool calls, and five minutes. Invalid tool names/arguments return corrective tool results; infrastructure errors, incomplete responses, cancellation or exhausted budgets fail the run through the existing retry/state path. A final plan must contain either a reply or a specific skip reason. A plan with neither receives correction within the same bounded loop, without saving its partial changes.

Behavioral coverage runs PostgreSQL → Agent → official SDK → HTTP model fixture → real reads → saved draft. It covers no Company root, page and conversation pagination, parent references, complete long pages and quoted history, multiple calls per response, retained provider fields and cache prefixes, denied writes, workspace isolation, argument rejection, incomplete/refused responses, cancellation, backend failure, both budgets, and correction of an unfinished final plan. Two temporary negative controls removed reasoning replay and argument validation independently; the regression test failed at the intended assertions.

Live verification used only synthetic records in disposable local databases with `gpt-6-luna`/medium. Two completed runs made three and five reads respectively, then drafted the retrieved GBP 42 price. Both required one correction of a model answer containing neither reply nor skip reason, which is why final-outcome validation is part of the loop. Provider usage showed cache reuse within the loop (3,432 of 3,547 input tokens on one continuation) and across identical initial requests (2,134 of 2,137 input tokens). These are observed samples, not a promised cache-hit rate or draft-quality benchmark. No production records were changed by these probes.

Review: retrieval policy and projections stay in followups; the SDK loop owns protocol replay and caching. Existing record and interaction services retain authorization, filtering and original-text ownership. No new dependency, MCP transport loopback, persistence layer or frontend controls are added.

Final verification: Go 1.26 `go build ./...`, `go vet ./...`, and the full backend `go test ./...` passed with PostgreSQL. All disposable test databases were removed. The strict maintainability review found no remaining blocker.
