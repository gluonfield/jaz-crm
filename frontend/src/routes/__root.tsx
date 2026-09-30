import { type QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { HeadContent, Outlet, Scripts, createRootRouteWithContext, useRouter } from '@tanstack/react-router'
import { type ReactNode, useEffect } from 'react'
import { TooltipProvider } from '@/components/ui/tooltip'
import { installHostBridge } from '@/lib/theme'
import styles from '@/styles.css?url'

// Applies the saved or system scheme before first paint to avoid a flash.
const schemeScript = `(function(){try{var p=localStorage.getItem('jaz-crm:scheme')||'system';var t=new URLSearchParams(location.search).get('theme');var d=t==='dark'||(t!=='light'&&(p==='dark'||(p==='system'&&matchMedia('(prefers-color-scheme: dark)').matches)));document.documentElement.classList.toggle('dark',d)}catch(e){}})()`

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  head: () => ({
    meta: [{ charSet: 'utf-8' }, { name: 'viewport', content: 'width=device-width, initial-scale=1' }, { title: 'Jaz CRM' }],
    links: [{ rel: 'stylesheet', href: styles }],
  }),
  shellComponent: Shell,
  component: Root,
})

function Shell({ children }: { children: ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: schemeScript }} />
        <HeadContent />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  )
}

function Root() {
  const { queryClient } = useRouter().options.context
  useEffect(installHostBridge, [])
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider delayDuration={500}>
        <Outlet />
      </TooltipProvider>
    </QueryClientProvider>
  )
}
