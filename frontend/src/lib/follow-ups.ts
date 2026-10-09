import { textOf } from './crm'
import { isDue } from './dates'
import type { CrmRecord } from './types'

export const statusOf = (record: CrmRecord) => textOf(record, 'status') || 'Open'

export type Standing = 'reply' | 'todo' | 'chase' | 'waiting' | 'done' | 'dismissed'

// standingOf is whose move a follow-up is: ours to reply to or do, theirs
// until the day to chase them comes, or closed.
export function standingOf(record: CrmRecord, zone: string, now = new Date()): Standing {
  const status = statusOf(record)
  if (status !== 'Open') {
    return status === 'Done' ? 'done' : 'dismissed'
  }
  if (textOf(record, 'waiting_on') === 'Them') {
    const due = textOf(record, 'action_date')
    return due && isDue(due, zone, now) ? 'chase' : 'waiting'
  }
  return textOf(record, 'draft') ? 'reply' : 'todo'
}
