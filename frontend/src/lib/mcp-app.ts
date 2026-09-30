import { App, type McpUiHostContext, PostMessageTransport, applyDocumentTheme, applyHostStyleVariables } from '@modelcontextprotocol/ext-apps'
import { applyTheme } from './theme'

// The Jaz CRM MCP App view, connected to the host that embeds the
// ui://jaz-crm/app resource.
export const app = new App({ name: 'Jaz CRM', version: '0.1.0' }, { availableDisplayModes: ['fullscreen'] })

// Hosts, Jaz included, theme apps with the spec's standard variables, which
// the stylesheet's tokens use as fallbacks (see THEMING.md).
export function applyHostContext(context: McpUiHostContext | undefined) {
  if (context?.theme) {
    applyDocumentTheme(context.theme)
    applyTheme({ scheme: context.theme })
  }
  if (context?.styles?.variables) {
    applyHostStyleVariables(context.styles.variables)
  }
}

export async function connect() {
  app.onhostcontextchanged = applyHostContext
  await app.connect(new PostMessageTransport(window.parent, window.parent))
  applyHostContext(app.getHostContext())
}

// callTool runs a tool through the host; its output is the structured content.
export async function callTool(name: string, args: object) {
  const result = await app.callServerTool({ name, arguments: args as Record<string, unknown> })
  if (result.isError) {
    const text = result.content.find((c) => c.type === 'text')
    throw new Error(text && 'text' in text ? text.text : `${name} failed`)
  }
  return result.structuredContent
}
