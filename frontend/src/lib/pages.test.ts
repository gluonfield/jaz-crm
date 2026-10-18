import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { CrmRecord } from './types'
import { pagesOf, slot } from './pages'

const page = (id: string, minute: number, values: CrmRecord['values'] = {}): CrmRecord => ({
  id,
  object: 'pages',
  created_at: `2026-10-13T12:0${minute}:00Z`,
  updated_at: `2026-10-13T12:0${minute}:00Z`,
  values: { name: id, ...values },
})

const at = (minute: number) => Date.parse(`2026-10-13T12:0${minute}:00Z`)

test('pages sort by stored position, else by when they were made', () => {
  const pages = pagesOf([
    page('a', 0),
    page('b', 1),
    page('c', 2, { position: String((at(0) + at(1)) / 2) }),
    page('lost', 3, { parent: { id: 'gone', name: 'Gone' } }),
  ], [])
  assert.deepEqual(pages.children.get('')?.map((p) => p.id), ['a', 'c', 'b', 'lost'])
  assert.equal(pages.byId.get('lost')?.parent, undefined, 'a page whose parent is gone sits at the top level')
})

test('a dropped page takes an order between its new neighbours', () => {
  const pages = pagesOf([page('a', 0), page('b', 1), page('c', 2), page('d', 3, { parent: { id: 'a', name: 'a' } })], [])
  assert.deepEqual(slot(pages, 'c', { id: 'a', place: 'before' }), { parent: '', order: at(0) - 1000 })
  assert.deepEqual(slot(pages, 'c', { id: 'a', place: 'after' }), { parent: '', order: (at(0) + at(1)) / 2 })
  assert.deepEqual(slot(pages, 'a', { id: 'c', place: 'after' }), { parent: '', order: at(2) + 1000 })
  assert.deepEqual(slot(pages, 'c', { id: 'd', place: 'before' }), { parent: 'a', order: at(3) - 1000 }, 'beside a sub-page moves into its parent')
  assert.deepEqual(slot(pages, 'b', { id: 'a', place: 'inside' }), { parent: 'a', order: at(3) + 1000 }, 'inside a page goes last')
  assert.deepEqual(slot(pages, 'd', { id: '', place: 'inside' }), { parent: '', order: at(2) + 1000 }, 'to the top level goes last')
})
