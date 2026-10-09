import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { defaultParseSearch, defaultStringifySearch } from '@tanstack/react-router'
import { filterKeys, recordSearchInput, validateRecordSearch } from './record-search'
import type { RecordFilter } from './types'

describe('conversation search links', () => {
  const conversation = 'e01c5237-f67b-4033-a91f-21589b0708c8'
  const raw = `ui://jaz-crm/o/follow_ups?conversation_id=${conversation}&filters=%5B%5D&group_by_conversation=false&limit=20&view=table`
  const grouped = 'ui://jaz-crm/o/follow_ups?filters=%5B%5D&group_by_conversation=true&limit=20&q=%22Done%20action%22'

  test('history reload retains its conversation and every action status', () => {
    const input = recordSearchInput(raw)
    assert.deepEqual(input, { object: 'follow_ups', conversation_id: conversation, group_by_conversation: false, filters: [], query: '', limit: 20 })
    const route = validateRecordSearch(defaultParseSearch(new URL(raw).search))
    assert.equal(route.view, 'table')
    assert.deepEqual(route.filters, [])
    assert.equal(route.conversation_id, conversation)
    const cleared = validateRecordSearch({ ...route, conversation_id: undefined })
    assert.equal(cleared.conversation_id, undefined)
    assert.equal(cleared.view, 'table')
    assert.deepEqual(cleared.filters, [])
  })

  test('grouped searches keep grouping and historical text matching', () => {
    const input = recordSearchInput(grouped)
    assert.deepEqual(input, { object: 'follow_ups', group_by_conversation: true, conversation_id: undefined, filters: [], query: 'Done action', limit: 20 })
  })
})

test('last updated sorting survives route reload and embedded search links', () => {
  const path = '/o/people?sort=updated_at'
  const route = validateRecordSearch(defaultParseSearch(new URL(path, 'http://crm').search))
  assert.equal(route.sort, 'updated_at')
  assert.equal(recordSearchInput(path).sort, 'updated_at')
  assert.equal(validateRecordSearch({ ...route, sort: undefined }).sort, undefined)
})

describe('filter links', () => {
  const read = (url: string) => validateRecordSearch(defaultParseSearch(new URL(url, 'http://crm').search)).filters
  const sorted = (filters: RecordFilter[] = []) => filters.map((f) => JSON.stringify(f)).sort()

  test('filters read as their own keys and come back unchanged', () => {
    const filters: RecordFilter[] = [
      { attribute: 'tags', operator: 'is', value: 'Advisor Candidate' },
      { attribute: 'updated_at', operator: 'after', value: '2026-10-01' },
      { attribute: 'links', operator: 'is_not_empty' },
    ]
    const search = defaultStringifySearch(filterKeys(filters))
    assert.equal(search, '?tags=Advisor+Candidate&updated_at.after=2026-10-01&links.is_not_empty=')
    assert.deepEqual(read(`/o/people${search}`), filters)
  })

  test('links the CRM writes for agents open the same list', () => {
    const input = recordSearchInput('ui://jaz-crm/o/people?employee_count.after=%2242%22&limit=20&q=Stone&tags=Founder&tags=Manufacturing')
    assert.equal(input.query, 'Stone')
    assert.deepEqual(input.filters, [
      { attribute: 'employee_count', operator: 'after', value: '42' },
      { attribute: 'tags', operator: 'is', value: 'Founder' },
      { attribute: 'tags', operator: 'is', value: 'Manufacturing' },
    ])
  })

  test('conditions keys cannot express keep their meaning', () => {
    const filters: RecordFilter[] = [
      { attribute: 'tags', operator: 'is', value: 'A' },
      { attribute: 'tags', operator: 'is', value: 'B' },
      { attribute: 'view', operator: 'is', value: 'Board' },
    ]
    assert.deepEqual(sorted(read(`/o/people${defaultStringifySearch(filterKeys(filters))}`)), sorted(filters))
  })

  test('a filter read from the URL neither doubles nor outlives its removal', () => {
    for (const url of ['?tags=Advisor+Candidate', '?links.is_not_empty']) {
      const raw = defaultParseSearch(url)
      // The router keeps the URL's keys beside the validated search.
      const current = { ...raw, ...validateRecordSearch(raw) }
      assert.deepEqual(validateRecordSearch(current).filters, validateRecordSearch(raw).filters)
      assert.equal(validateRecordSearch(current).filters?.length, 1)
      assert.equal(validateRecordSearch({ ...current, filters: undefined }).filters, undefined)
    }
  })

  test('older JSON filter links still open the same list', () => {
    const json = '/o/people?filters=%5B%7B%22attribute%22%3A%22tags%22%2C%22operator%22%3A%22is%22%2C%22value%22%3A%22Advisor%20Candidate%22%7D%5D'
    assert.deepEqual(read(json), read('/o/people?tags=Advisor+Candidate'))
  })
})
