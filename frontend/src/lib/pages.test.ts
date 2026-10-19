import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { CrmObject, CrmRecord } from './types'
import { type Item, type Pages, allowed, isTable, pagesOf, slot } from './pages'

const page = (id: string, minute: number, values: CrmRecord['values'] = {}): CrmRecord => ({
  id,
  object: 'pages',
  created_at: `2026-10-13T12:0${minute}:00Z`,
  updated_at: `2026-10-13T12:0${minute}:00Z`,
  values: { name: id, ...values },
})

const table = (slug: string, position: number, parent?: string): CrmObject => ({ slug, name: slug, parent, position, attributes: [] })

const at = (minute: number) => Date.parse(`2026-10-13T12:0${minute}:00Z`)

const name = (item: Item) => (isTable(item) ? item.slug : item.id)

const find = (pages: Pages, wanted: string) => [...pages.children.values()].flat().find((item) => name(item) === wanted)!

test('pages and tables sort together by position, a page without one by when it was made, and an item whose parent is gone at the top level', () => {
  const pages = pagesOf([
    page('a', 0),
    page('b', 1),
    page('c', 2, { position: String((at(0) + at(1)) / 2) }),
    page('lost', 3, { parent: { id: 'gone', name: 'Gone' } }),
  ], [table('events', at(1) + 1), table('stray', at(0) - 1, 'gone'), table('calls', 0, 'a'), { slug: 'people', name: 'People', standard: true, position: 0, attributes: [] }])
  assert.deepEqual(pages.children.get('')?.map(name), ['stray', 'a', 'c', 'b', 'events', 'lost'])
  assert.deepEqual(pages.children.get('a')?.map(name), ['calls'])
})

test('a dropped page or table takes a position between its new neighbours', () => {
  const pages = pagesOf([page('a', 0), page('b', 1), page('c', 2), page('d', 3, { parent: { id: 'a', name: 'a' } })], [table('events', at(1) + 1)])
  const [a, b, c, d, events] = ['a', 'b', 'c', 'd', 'events'].map((wanted) => find(pages, wanted))
  assert.deepEqual(slot(pages, c, { item: a, place: 'before' }), { parent: '', position: at(0) - 1000 })
  assert.deepEqual(slot(pages, c, { item: a, place: 'after' }), { parent: '', position: (at(0) + at(1)) / 2 })
  assert.deepEqual(slot(pages, a, { item: c, place: 'after' }), { parent: '', position: at(2) + 1000 })
  assert.deepEqual(slot(pages, events, { item: a, place: 'before' }), { parent: '', position: at(0) - 1000 }, 'a table moves above a page')
  assert.deepEqual(slot(pages, c, { item: events, place: 'before' }), { parent: '', position: at(1) + 0.5 }, 'a page moves above a table')
  assert.deepEqual(slot(pages, events, { item: d, place: 'before' }), { parent: 'a', position: at(3) - 1000 }, 'beside a sub-page moves into its parent')
  assert.deepEqual(slot(pages, b, { place: 'inside', page: pages.byId.get('a') }), { parent: 'a', position: at(3) + 1000 }, 'inside a page goes last')
  assert.deepEqual(slot(pages, d, { place: 'inside' }), { parent: '', position: at(2) + 1000 }, 'to the top level goes last')
})

test('a move changes where an item sits and never puts a page inside its own branch', () => {
  const pages = pagesOf([page('a', 0), page('b', 1), page('d', 3, { parent: { id: 'a', name: 'a' } })], [table('events', 0, 'a')])
  const [a, b, d, events] = ['a', 'b', 'd', 'events'].map((wanted) => find(pages, wanted))
  assert.equal(allowed(pages, d, { place: 'inside', page: pages.byId.get('a') }), false, 'inside its own parent')
  assert.equal(allowed(pages, b, { place: 'before', item: b }), false, 'beside itself')
  assert.equal(allowed(pages, a, { place: 'inside', page: pages.byId.get('d') }), false, 'inside its own branch')
  assert.equal(allowed(pages, a, { place: 'after', item: d }), false, 'beside its own sub-page')
  assert.equal(allowed(pages, events, { place: 'inside', page: pages.byId.get('b') }), true)
  assert.equal(allowed(pages, b, { place: 'after', item: events }), true)
})
