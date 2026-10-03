import assert from 'node:assert/strict'
import { test } from 'node:test'
import { dateValue, zonedInput } from './dates'

test('editing dates preserves optional precision and handles London clock changes', () => {
  assert.equal(dateValue('2026-10-03', '', 'Europe/London'), '2026-10-03')
  assert.equal(dateValue('2026-10-03', '19:00', 'Europe/London'), '2026-10-03T18:00:00.000Z')
  assert.equal(dateValue('2026-10-26', '19:00', 'Europe/London'), '2026-10-26T19:00:00.000Z')
  assert.throws(() => dateValue('2026-03-29', '01:30', 'Europe/London'), /does not exist/)
  assert.equal(dateValue('2026-10-25', '01:30', 'Europe/London', '2026-10-25T01:30:00Z'), '2026-10-25T01:30:00Z')
  assert.equal(dateValue('2026-10-25', '01:30', 'Europe/London', '2026-10-25T00:30:00Z'), '2026-10-25T00:30:00Z')
  assert.equal(zonedInput('2026-10-03T23:00:00Z', 'Europe/London'), '2026-10-04T00:00')
  assert.equal(dateValue('2026-10-04', '00:00', 'Asia/Kathmandu'), '2026-10-03T18:15:00.000Z')
})
