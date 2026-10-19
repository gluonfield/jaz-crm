import { createRoot } from 'react-dom/client'
import { createMemoryHistory, createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { DataNav } from '../src/components/data-nav'
import { newQueryClient } from '../src/lib/queries'

const router = createRouter({ routeTree: createRootRoute({ component: DataNav }), history: createMemoryHistory() })
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={newQueryClient()}><RouterProvider router={router} /></QueryClientProvider>)
