import assert from 'node:assert/strict'
import { test } from 'node:test'
import { QueryClientProvider } from '@tanstack/react-query'
import { createMemoryHistory, createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router'
import { renderToStaticMarkup } from 'react-dom/server'
import { newQueryClient } from '@/lib/queries'
import type { CrmObject, CrmRecord } from '@/lib/types'
import { Field } from './fields'
import { RecordCard } from './record-card'
import { Related } from './related'
import { Summary } from './summary'

test('a person summary keeps short notes and context in their own fields', () => {
  const object: CrmObject = { slug: 'people', name: 'People', attributes: [
    { slug: 'name', name: 'Name', type: 'text' },
    { slug: 'job_title', name: 'Job title', type: 'text' },
    { slug: 'notes', name: 'Notes', type: 'text' },
    { slug: 'context', name: 'Context', type: 'text' },
  ] }
  const record: CrmRecord = {
    id: 'ada', object: 'people', created_at: '2026-10-08T12:00:00Z', updated_at: '2026-10-08T12:00:00Z',
    values: { name: 'Ada', job_title: 'Engineer', notes: 'Introduced by a customer', context: 'Evaluating a factory project' },
  }
  const client = newQueryClient()
  const html = renderToStaticMarkup(<QueryClientProvider client={client}><Summary record={record} object={object} name="Ada" upcoming={[]}><h2>Ada</h2></Summary></QueryClientProvider>)
  assert.match(html, /Ada/)
  assert.match(html, /Engineer/)
  assert.doesNotMatch(html, /Introduced by a customer|Evaluating a factory project/)
  client.clear()
})

test('a teammate in a summary is named with their field', () => {
  const object: CrmObject = { slug: 'people', name: 'People', attributes: [
    { slug: 'name', name: 'Name', type: 'text' },
    { slug: 'owner', name: 'Owner', type: 'member' },
  ] }
  const record: CrmRecord = {
    id: 'andy', object: 'people', created_at: '2026-10-09T12:00:00Z', updated_at: '2026-10-09T12:00:00Z',
    values: { name: 'Andy', owner: 'august@jaz.test' },
  }
  const client = newQueryClient()
  const html = renderToStaticMarkup(<QueryClientProvider client={client}><Summary record={record} object={object} name="Andy" upcoming={[]}><h2>Andy</h2></Summary></QueryClientProvider>)
  assert.match(html, />Owner<\/span>.*august@jaz\.test/)
  client.clear()
})

test('link fields open their pages from a record', () => {
  const record: CrmRecord = {
    id: 'irwin', object: 'people', created_at: '2026-10-09T12:00:00Z', updated_at: '2026-10-09T12:00:00Z',
    values: { links: ['https://www.linkedin.com/in/irwin-zaid/'], website: 'https://oxfordedge.ox.ac.uk/', domains: ['ox.ac.uk'] },
  }
  const client = newQueryClient()
  const render = (attribute: CrmObject['attributes'][number]) => renderToStaticMarkup(<QueryClientProvider client={client}><Field record={record} attribute={attribute} /></QueryClientProvider>)
  const links = render({ slug: 'links', name: 'Links', type: 'url', multi: true })
  assert.match(links, /<a href="https:\/\/www\.linkedin\.com\/in\/irwin-zaid\/"[^>]*target="_blank"[^>]*>linkedin\.com\/in\/irwin-zaid\/<\/a>/)
  assert.match(links, /<input/)
  const website = render({ slug: 'website', name: 'Website', type: 'url' })
  assert.match(website, /<a href="https:\/\/oxfordedge\.ox\.ac\.uk\/"[^>]*>oxfordedge\.ox\.ac\.uk<\/a>/)
  assert.doesNotMatch(website, /<input/)
  assert.match(render({ slug: 'domains', name: 'Domains', type: 'domain', multi: true }), /<a href="https:\/\/ox\.ac\.uk"[^>]*>ox\.ac\.uk<\/a>/)
  client.clear()
})

test('page decorations leave custom Icon fields visible in record cards', () => {
  for (const [object, icon] of [['parts', 'Brand asset #2'], ['companies', 'Brand asset #2'], ['pages', 'icon:rocket'], ['pages', 'icon:Drill:blue'], ['pages', 'image:https://example.com/logo.png']]) {
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
      assert.doesNotMatch(html, /icon:(rocket|Drill)/)
      assert.doesNotMatch(html, /image:http/)
      if (icon.startsWith('image:')) {
        assert.match(html, /<img[^>]+src="https:\/\/example.com\/logo.png"/)
      }
      if (icon === 'icon:Drill:blue') {
        assert.match(html, /lucide-drill/)
        assert.match(html, /color:var\(--page-icon-blue\)/)
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
