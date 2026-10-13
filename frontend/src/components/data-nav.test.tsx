import assert from 'node:assert/strict'
import { test } from 'node:test'
import { QueryClientProvider } from '@tanstack/react-query'
import { createMemoryHistory, createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router'
import { renderToStaticMarkup } from 'react-dom/server'
import { newQueryClient } from '@/lib/queries'
import type { CrmObject, CrmRecord } from '@/lib/types'
import { DataNav } from './data-nav'

test('root and nested sidebar databases expose the same drag affordance as pages', async () => {
  const records: CrmRecord[] = [
    { id: 'company', object: 'pages', created_at: '2026-10-13T12:00:00Z', updated_at: '2026-10-13T12:00:00Z', values: { name: 'Company' } },
    { id: 'customers', object: 'pages', created_at: '2026-10-13T12:01:00Z', updated_at: '2026-10-13T12:01:00Z', values: { name: 'Customers', parent: { id: 'company', name: 'Company' } } },
  ]
  const objects: CrmObject[] = [
    { slug: 'problems', name: 'Industry Problems', attributes: [] },
    { slug: 'customer_calls', name: 'Customer Calls', parent: 'customers', attributes: [] },
  ]
  const client = newQueryClient()
  client.setQueryData(['search_records', { object: 'pages', limit: 500 }], { records })
  client.setQueryData(['list_objects', {}], { objects })
  const route = createRootRoute({ component: DataNav })
  const router = createRouter({ routeTree: route, history: createMemoryHistory({ initialEntries: ['/o/customer_calls'] }) })
  await router.load()
  const html = renderToStaticMarkup(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
  for (const href of ['/o/problems', '/o/customer_calls', '/r/company', '/r/customers']) {
    const row = html.match(new RegExp(`<div([^>]*)><a[^>]*href="${href}"`))
    assert.ok(row, `${href} is visible in the sidebar`)
    assert.match(row[1], /\bdraggable="true"/, `${href} can start a drag`)
  }
  client.clear()
})
