import type { McpUiHostContext } from '@modelcontextprotocol/ext-apps'
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router'
import { createRoot } from 'react-dom/client'
import { setTransport } from './lib/api'
import { app, callTool, connect } from './lib/mcp-app'
import { newQueryClient } from './lib/queries'
import { routeTree } from './routeTree.gen'
import { Route as root } from './routes/__root'
import './styles.css'

// The MCP App renders the web app's routes inside a host's sandboxed,
// opaque-origin iframe: no document shell or sign-in, in-memory history,
// tools called through the host, and external links opened by the host.
;(root.options as { shellComponent?: unknown }).shellComponent = undefined
setTransport(callTool)

const router = createRouter({
  routeTree,
  history: createMemoryHistory({ initialEntries: [document.getElementById('root')?.dataset.startPath ?? '/'] }),
  context: { queryClient: newQueryClient() },
})

function open(path: unknown) {
  if (typeof path !== 'string') {
    return
  }
  let href = path
  if (path.startsWith('ui://jaz-crm/')) {
    const uri = new URL(path)
    href = uri.pathname + uri.search
  }
  if (href.startsWith('/')) {
    void router.navigate({ href })
  }
}

// show_crm passes the page to open, such as /r/<record id>.
app.ontoolinput = ({ arguments: args }) => open(args?.path)
app.ontoolresult = (result) => {
  const content = result.structuredContent
  if (content && typeof content === 'object' && 'resource_uri' in content) {
    open(content.resource_uri)
  }
}

// A host's deep link opens the app at a page, at start and whenever the host
// changes it.
function follow(context?: McpUiHostContext) {
  open((context?.['openai/deepLink'] as { url?: unknown } | undefined)?.url)
}
app.addEventListener('hostcontextchanged', follow)

document.addEventListener('click', (event) => {
  const link = (event.target as HTMLElement).closest<HTMLAnchorElement>('a[href^="http"]')
  if (link) {
    event.preventDefault()
    void app.openLink({ url: link.href })
  }
})

void connect().finally(() => {
  follow(app.getHostContext())
  createRoot(document.getElementById('root')!).render(<RouterProvider router={router} />)
})
