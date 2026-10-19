# Push Updates

- [x] Inventory every timer-driven request and the change it waits for.
- [x] Replace polling with a push channel: open pages refetch only when their workspace's data changed, from the browser, an agent over MCP or the sync worker.
- [x] Keep at most bounded or slow refetches where no write marks the change.
- [x] Find and fix the cause of the slow first `search_records` on Companies.
- [x] Tests with negative controls; idle request counts and change latency in a hidden browser.
- [ ] Approval, merge main, push and verify the deployment and idle requests on crm.jaz.chat.

## Polls

| Before | Waited for | Now |
| --- | --- | --- |
| `list_saved_filters` every 5 s (two components) | another tab, member or agent saving a view | push |
| `list_workspaces` every 10 s | a workspace switch or rename elsewhere | own switches refetch; other tabs on focus |
| `list_connections` every 30 s, 3 s while syncing | sync progress | push; every 3 s only while a sync step shows, as the step lives in Temporal; every 60 s on Connections, whose sync times move each pass |
| invalidate everything every 5 s while syncing | mail and meetings arriving | push |
| `follow_ups` pages every 60 s | Chase's `on_or_before now` filling with time | kept: time-driven |
| `get_interaction` every 3 s while drafting | the draft agent finishing | push |

## Design

- One server replica and one worker process on Railway behind Cloudflare, one Postgres. Sync writes happen in the worker, so Postgres is the one place every write passes: statement-level triggers on each workspace table call `pg_notify('changes', workspace_id)` at commit. Updates that rewrite rows as they were notify nothing; production showed idle passes rewriting about 21 Gmail draft rows a minute.
- The server's hub LISTENs on a connection of its own and fans out by workspace. `GET /api/changes` is a session-authenticated server-sent event stream: a comment every 30 s for Cloudflare's 100 s idle limit, one event per change, at least 5 s apart so a sync burst refetches a page at most that often. Railway ends HTTP requests after 15 minutes; the browser reconnects and the page refetches once, as it may have missed changes. If the hub's connection drops, every page refetches once it listens again.
- Hidden tabs only mark data stale and refetch when shown. Server-sent events need no dependency; WebSockets would have needed a library.

## search_records

- The final SELECT joined `records` again although the CTE carried its rows. In pgx's first five executions on each pooled connection, Postgres plans with the parameters; the filters' anti-join then estimated 1 to 3 rows and the re-join became a nested loop scanning `records` for each match: 2,822 × 3,719 rows filtered, 1,089 ms in production's database. Generic plans took about 50 ms, which matches production's bimodal 80–100 ms and 350–800 ms timings.
- Selecting the carried columns removes the join. On a production copy with a fresh server, the Companies list took 430 ms for its first five calls and 44 ms after; now 30 ms and 41 ms. Planned with parameters: 460 ms → 3.4 ms; generically: 20 ms → 19 ms.

## Verification

- Hidden headless Chromium shell against a production copy. Before: 20 requests in 60 s idle locally (production showed 32 in 52 s with a live sync), and a company added by another session did not appear within 15 s. After: no requests in 120 s idle; the company appeared 139 ms after the write; a change made while the page's server was down appeared 596 ms after it restarted.
- Shutdown with an open stream takes 0.08 s; without closing streams on shutdown it waits out the 60 s stop timeout.
- Trigger cost: about 0.09 ms per write statement; a no-op rewrite of all 14,304 message parts takes 915 ms instead of 627 ms.
- Negative controls, each failing the tests for the intended reason: no notify, no no-op filter, no delete trigger, events to every workspace, streams surviving the hub.
