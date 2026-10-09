import assert from 'node:assert/strict'
import { test } from 'node:test'
import { formatDay } from './format'
import { dateValue, dueLabel, isDue, zonedInput } from './dates'

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

test('due dates count days where the workspace is', () => {
  const now = new Date('2026-10-08T23:30:00Z')
  assert.deepEqual(dueLabel('2026-10-07', 'Europe/London', now), { label: 'Overdue 2d', late: true })
  assert.deepEqual(dueLabel('2026-10-08T21:00:00Z', 'Europe/London', now), { label: 'Overdue 1d', late: true })
  assert.deepEqual(dueLabel('2026-10-09T20:00:00Z', 'Europe/London', now), { label: 'Today', late: false })
  assert.deepEqual(dueLabel('2026-10-09', 'UTC', now), { label: 'Tomorrow', late: false })
  assert.deepEqual(dueLabel('2026-10-15', 'Europe/London', now), { label: formatDay('2026-10-15'), late: false })
  assert.equal(isDue('2026-10-09', 'Europe/London', now), true)
  assert.equal(isDue('2026-10-09', 'UTC', now), false)
  assert.equal(isDue('2026-10-09T20:00:00Z', 'Europe/London', now), false)
  assert.equal(isDue('2026-10-08T23:00:00Z', 'Europe/London', now), true)
})
