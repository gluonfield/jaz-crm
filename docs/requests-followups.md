# Follow-ups and filter header

- [x] Use On hold for interested deals that should be revisited later.
- [x] Add Next follow-up date and Next action to deals; show the date on board cards.
- [x] Seed Due follow-ups as an ordinary editable/deletable saved filter for existing and new workspaces.
- [x] Match follow-up dates on or before dynamic Today, excluding Won and Lost.
- [x] Keep deleted defaults deleted across reloads and restarts.
- [x] Move Filters into the existing header on every record page.
- [x] Review and run backend build/vet/full tests and frontend typecheck/lint/build.
- [x] Commit, push and check local/hosted activation.

Real PostgreSQL/MCP regressions cover existing-workspace upgrade, preservation of custom stages/deals, due/overdue/future/empty dates, closed-deal exclusion, rescheduling, workspace isolation, filter editing and persistent deletion. A Go overlay changing <= to < makes the equality regression fail without changing product source.

The filter editor saves its current draft directly. Saved-filter controls share the existing header; condition editing, saving and deletion live in the popover.

Visual acceptance is pending because the Jaz side browser is disconnected.

Implementation `8248e19` is pushed to main. GitHub run [36873433274](https://github.com/gluonfield/jaz-crm/actions/runs/36873433274) passed verification and published both images. Both Railway services reached SUCCESS for that commit; CAS MCP reads confirm the stage, fields and usable default filter. Local server/worker run images labelled with the same revision; all three existing local deal schemas contain the additions, migration 15 is applied, and local/hosted health checks return 200. No shared CRM records were changed for verification.
