import { useEffect } from 'react'
import type { McpUiHostContext } from '@modelcontextprotocol/ext-apps'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router'
import { createRoot } from 'react-dom/client'
import { RecordCard } from './components/record-card'
import { RecordResults } from './components/record-results'
import { setTransport } from './lib/api'
import { app, callTool, connect } from './lib/mcp-app'
import { newQueryClient, toolQuery } from './lib/queries'
import { recordSearchInput } from './lib/record-search'
import { routeTree } from './routeTree.gen'
import { Route as root } from './routes/__root'
import './styles.css'

// The MCP App renders the web app's routes inside a host's sandboxed,
// opaque-origin iframe: no document shell or sign-in, in-memory history,
// tools called through the host, and external links opened by the host.
;(root.options as { shellComponent?: unknown }).shellComponent = undefined
setTransport(callTool)

const container = document.getElementById('root')!
const renderer = createRoot(container)
const queryClient = newQueryClient()
let page = container.dataset.startPath ?? '/'
let ready = false
const router = createRouter({
  routeTree,
  history: createMemoryHistory({ initialEntries: [page] }),
  context: { queryClient },
})

function reportNavigation(replace: boolean, delta?: number) {
  if (!app.getHostCapabilities()?.experimental?.['jaz/navigation'] || app.getHostContext()?.displayMode !== 'fullscreen') return
  const path = router.history.location.href
  void app.notification({ method: 'jaz/notifications/navigation', params: { path, replace, ...(delta && { delta }) } }).catch(console.error)
}

// Subscribe after RouterProvider mounts: the router treats history subscribers as its loading adapter.
function AppRouter() {
  useEffect(() => {
    reportNavigation(true)
    return router.history.subscribe(({ location, action }) => {
      const delta = action.type === 'BACK' ? -1 : action.type === 'FORWARD' ? 1 : action.type === 'GO' ? action.index : undefined
      const target = (app.getHostContext()?.['jaz/navigation'] as { path?: string } | undefined)?.path
      if (delta || location.href !== target) reportNavigation(action.type !== 'PUSH', delta)
    })
  }, [])
  return <RouterProvider router={router} />
}

function render() {
  if (!ready) {
    return
  }
  if (app.getHostContext()?.displayMode === 'inline') {
    const recordId = new URL(page, 'http://crm').pathname.match(/^\/r\/([^/]+)$/)?.[1]
    renderer.render(
      <QueryClientProvider client={queryClient}>
        {recordId ? (
          <RecordCard recordId={decodeURIComponent(recordId)} onOpen={(url) => void app.openLink({ url })} />
        ) : (
          <RecordResults path={page} onOpen={(url) => void app.openLink({ url })} />
        )}
      </QueryClientProvider>,
    )
  } else {
    renderer.render(<AppRouter />)
    void router.navigate({ href: page }).then(() => reportNavigation(true))
  }
}

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
    page = href
    render()
  }
}

app.ontoolinput = ({ arguments: args }) => open(args?.path)
app.ontoolresult = (result) => {
  const content = result.structuredContent
  if (content && typeof content === 'object' && 'resource_uri' in content && typeof content.resource_uri === 'string') {
    if ('records' in content && Array.isArray(content.records)) {
      queryClient.setQueryData(toolQuery('search_records', recordSearchInput(content.resource_uri)).queryKey, content)
    } else if ('id' in content && typeof content.id === 'string') {
      queryClient.setQueryData(toolQuery('get_record', { record_id: content.id }).queryKey, content)
    }
    if (!app.getHostCapabilities()?.experimental?.['jaz/navigation'] || app.getHostContext()?.displayMode === 'inline') open(content.resource_uri)
  }
}

// A host's deep link opens the app at a page, at start and whenever the host
// changes it.
function follow(context?: McpUiHostContext) {
  const path = (context?.['jaz/navigation'] as { path?: unknown } | undefined)?.path
  if (typeof path === 'string' && path.startsWith('/')) {
    page = path
    if (router.history.location.href !== path) void router.navigate({ href: path, replace: true })
    return
  }
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
  ready = true
  follow(app.getHostContext())
  render()
})
