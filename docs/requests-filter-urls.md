# Filter URLs

Augustinas found the CRM's filter links unreadable on 2026-10-09: `/o/people?filters=%5B%7B%22attribute%22…` is percent-encoded JSON, and a saved view's short link did not apply its filters.

- [x] Filters read as their own URL keys: `?tags=Advisor+Candidate`, `?updated_at.after=2026-10-01`, `?links.is_not_empty`.
- [x] `?saved=<id>` applies the saved view; choosing a view writes only that key.
- [x] The links the CRM returns to agents use the same keys.
- [x] Older JSON `filters` and `where` links still open the same list.

## Behaviour

- Every key except the list's own (`q`, `sort`, `view`, `limit`, `saved`, `filters`, `where`, `category`, `group_by_conversation`, `conversation_id`) is a filter: `attribute=value`, or `attribute.operator=value` for any other operator. Several conditions repeat a key.
- Keys naming no attribute of the object, such as a shared link's `utm_source`, filter nothing and do not replace a saved view.
- A condition the keys cannot express, such as a second value written by the app or an attribute named like a list key, stays in the JSON `filters` key. An empty `filters=[]` still means "no filters" where a list has defaults, as Follow-ups does.
- Text that would read back as JSON, such as `42`, is quoted, matching the router.

## Verification

- Go suite and vet pass; frontend tests, typecheck, lint and builds pass.
- Frontend tests cover the readable round trip, the exact links the backend writes (repeated keys, quoted numbers), the JSON fallback, the router's merge of URL keys with the validated search (a filter neither doubles nor survives removal, including operators without a value), and the original JSON link opening the same list. Negative controls fail without reading URL keys, without the duplicate-key fallback, without clearing read keys, and without skipping cleared keys.
- MCP tests assert `resource_uri` carries `q=Stone&tags=Founder&tags=Manufacturing` and `stage=Lead&company=acme.example`, and that reloading a resource URI reproduces its search.
- On a scratch server: the plain link, the saved-view link and the original JSON link each show the two Advisor Candidates; the JSON link rewrites itself to `?tags=Advisor+Candidate`; `utm_source` is ignored; choosing the view writes `?saved=<id>`; typing a search adds `&q=Gr`; removing a condition through the filter dialog leaves `?links.is_not_empty=`; Follow-ups keeps its defaults with a clean URL.
