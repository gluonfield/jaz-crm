# Embedded App Navigation

- [x] Report app page visits to Jaz's top Back/Forward controls.
- [x] Restore pages from Jaz history and preserve hosts without the capability.
- [x] Acknowledge search tool targets, including repeated opens.
- [x] Preserve in-app history traversal and normal router startup.

The MCP App opts into experimental `jaz/navigation`. Page changes report `jaz/notifications/navigation` with `path`, `replace` and optional `delta`; host context restores `jaz/navigation.path`. Jaz owns the chronological history across apps and chats.

Verified with the rebuilt app against live read-only MCP data in a production Jaz host, plus frontend checks/build and the full backend suite. Jaz's normal hidden desktop smoke covers the sandboxed protocol boundary and titlebar navigation, with a failing negative control. Strict review completed.
