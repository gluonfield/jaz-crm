# ChatGPT plugin

Jaz CRM uses its existing Streamable HTTP `/mcp` server and MCP Apps UI in ChatGPT. The same tools remain usable by other MCP clients and the web app.

## Implemented

- OAuth discovery, dynamic client registration, public-client token exchange, PKCE S256, refresh and revocation, and RFC 9207 issuer identification.
- Every tool declares OAuth in `_meta.securitySchemes` (the supported compatibility form for the Go SDK), with explicit read-only, destructive and open-world hints. Connections authorize the whole selected workspace; there are no narrower OAuth scopes.
- `get_profile` identifies the credential's current workspace membership. Its ID stays stable across token refresh and reconnection; switching workspace changes it to that workspace's membership ID.
- Self-contained UI resources use the standard MCP Apps bridge, declare their supported display modes and CSP, and use `PUBLIC_URL` as the component origin. Use a distinct origin for each plugin.
- `show_crm` is a global sidebar entrypoint. Availability depends on the ChatGPT surface and plan.
- `plugin/` contains the portable Agent Plugins manifest, icons and ZIP packager.

## Connect and test

1. Deploy the service at a stable HTTPS origin. Set `PUBLIC_URL` to that exact origin and configure the existing Google/OIDC sign-in callback at `PUBLIC_URL/auth/callback`.
2. In ChatGPT, enable Developer mode, open Plugins, and add the deployed `/mcp` URL. Choose OAuth with **dynamic client registration**, using the discovered authorization server and public-client (`none`) authentication. CIMD is not advertised or required.
3. Sign in through the existing provider and approve workspace access. The callback registered by ChatGPT is validated against its DCR client; successful and error callbacks include the issuer.
4. In a new chat, verify `get_profile`, list/search, a requested write, the app opening, navigation, and workspace isolation. Reconnect and refresh tokens, then check that the account ID remains stable. Check revoked credentials are rejected.
5. Package the endpoint only after it has passed that connection test:

```sh
python3 plugin/package.py "$PUBLIC_URL/mcp" /tmp/jaz-crm.zip
```

The archive contains `jaz-crm/plugin.json`, `mcp.json` and icons. Packaging does not deploy, register, authenticate or publish the plugin. It accepts an explicit endpoint rather than shipping a guessed deployment URL or credentials.

## Still required externally

- A reachable HTTPS deployment with production sign-in configured. Localhost alone is unavailable to ChatGPT's cloud MCP client; Secure MCP Tunnel is an option for private developer testing.
- A real ChatGPT connection and UI acceptance test. Local SDK integration tests establish protocol behavior, not host acceptance.
- For a public directory listing: verified endpoint domain, privacy policy, support details, review test accounts and test cases, and submission approval. The portable package is a private testing/distribution artifact; public review materials are a separate step.
- **Continue with ChatGPT** identity sign-in is optional and requires OpenAI trial access. Existing OAuth works independently.

Company knowledge search/fetch and enterprise OIDC domain restrictions are optional extensions; this integration does not advertise them.

## References

- [Plugin packaging](https://developers.openai.com/plugins/build/plugins)
- [Authentication](https://developers.openai.com/plugins/build/auth)
- [MCP Apps UI](https://developers.openai.com/plugins/build/chatgpt-ui)
- [Tool and component metadata](https://developers.openai.com/plugins/reference)
- [Connect to ChatGPT](https://developers.openai.com/plugins/deploy/connect-chatgpt)
- [Submit plugins](https://developers.openai.com/plugins/deploy/submission)
