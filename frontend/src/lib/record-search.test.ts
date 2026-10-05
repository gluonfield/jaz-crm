import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { defaultParseSearch } from '@tanstack/react-router'
import { recordSearchInput, validateRecordSearch } from './record-search'

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
