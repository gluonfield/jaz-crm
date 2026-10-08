import { createRouter } from '@tanstack/react-router'
import { Loading } from './components/controls'
import { newQueryClient } from './lib/queries'
import { routeTree } from './routeTree.gen'

export function getRouter() {
  return createRouter({
    routeTree,
    context: { queryClient: newQueryClient() },
    defaultPreload: 'intent',
    defaultPendingComponent: Loading,
    scrollRestoration: true,
  })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
