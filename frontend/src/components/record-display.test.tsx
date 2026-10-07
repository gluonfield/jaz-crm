import assert from 'node:assert/strict'
import { test } from 'node:test'
import { QueryClientProvider } from '@tanstack/react-query'
import { createMemoryHistory, createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router'
import { renderToStaticMarkup } from 'react-dom/server'
import { newQueryClient } from '@/lib/queries'
import type { CrmObject, CrmRecord } from '@/lib/types'
import { RecordCard } from './record-card'
import { Related } from './related'

test('page decorations leave custom Icon fields visible in record cards', () => {
  for (const [object, icon] of [['parts', 'Brand asset #2'], ['companies', 'Brand asset #2'], ['pages', 'icon:rocket'], ['pages', 'image:http://crm.test/page-icons/thumbnail']]) {
    const record: CrmRecord = {
      id: 'test-record', object, created_at: '2026-10-07T12:00:00Z', updated_at: '2026-10-07T12:00:00Z',
      values: { name: 'Valve', icon, content: 'Review before ordering' },
    }
    const client = newQueryClient()
    client.setQueryData(['get_record', { record_id: record.id }], record)
    const html = renderToStaticMarkup(<QueryClientProvider client={client}><RecordCard recordId={record.id} onOpen={() => {}} /></QueryClientProvider>)
    assert.match(html, /Valve/)
    assert.match(html, /Review before ordering/)
    if (object === 'pages') {
      assert.doesNotMatch(html, /icon:rocket/)
      assert.doesNotMatch(html, /image:http/)
      if (icon.startsWith('image:')) {
        assert.match(html, /<img[^>]+src="http:\/\/crm.test\/page-icons\/thumbnail"/)
      }
    } else {
      assert.match(html, /Brand asset #2/)
    }
    client.clear()
  }
})

test('a related custom table keeps its Icon field as descriptive data', async () => {
  const object: CrmObject = { slug: 'parts', name: 'Parts', attributes: [
    { slug: 'name', name: 'Name', type: 'text' },
    { slug: 'icon', name: 'Icon', type: 'text' },
    { slug: 'company', name: 'Company', type: 'reference', target: 'companies' },
  ] }
  const client = newQueryClient()
  client.setQueryData(['search_records', { object: 'parts', where: { company: 'acme' }, limit: 50 }], { records: [{
    id: 'part', object: 'parts', created_at: '2026-10-07T12:00:00Z', updated_at: '2026-10-07T12:00:00Z',
    values: { name: 'Valve', icon: 'Brand asset #2' },
  }] })
  const route = createRootRoute({ component: () => <Related recordId="acme" object="companies" objects={[object]} /> })
  const router = createRouter({ routeTree: route, history: createMemoryHistory({ initialEntries: ['/'] }) })
  await router.load()
  const html = renderToStaticMarkup(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
  assert.match(html, /Valve/)
  assert.match(html, /Brand asset #2/)
  client.clear()
})
