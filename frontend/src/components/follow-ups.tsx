import { LoaderCircle } from 'lucide-react'
import type { ReactNode } from 'react'
import { recordName, valuesOf } from '@/lib/crm'
import { isOverdue, zonedInput } from '@/lib/dates'
import { timeAgo } from '@/lib/format'
import { useWorkspace } from '@/lib/queries'
import { useConnections } from '@/lib/sync'
import type { CrmRecord, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { DateLabel } from './date-field'
import { Conversation, Done, refsOf, text } from './follow-up'
import { ChannelTag, RecordIcon } from './icons'

const oneDay = 86_400_000

const groups = [
  { name: 'Overdue', className: 'text-danger' },
  { name: 'Today', className: 'text-running' },
  { name: 'Next 7 days', className: 'text-ink-2' },
  { name: 'Later', className: 'text-ink-2' },
  { name: 'Done', className: 'text-ink-3' },
  { name: 'Dismissed', className: 'text-ink-3' },
]

// groupOf places a follow-up by when it is due where the workspace is;
// one without a date waits until later.
function groupOf(record: CrmRecord, zone: string) {
  const status = text(record, 'status')
  if (status === 'Done' || status === 'Dismissed') {
    return status === 'Done' ? 4 : 5
  }
  const due = text(record, 'action_date')
  if (!due) {
    return 3
  }
  if (isOverdue(due, zone)) {
    return 0
  }
  const today = zonedInput(new Date().toISOString(), zone).slice(0, 10)
  const day = due.length === 10 ? due : zonedInput(due, zone).slice(0, 10)
  return day === today ? 1 : Date.parse(day) - Date.parse(today) <= 7 * oneDay ? 2 : 3
}

// byUrgency orders follow-ups by group, keeping their order within each.
export const byUrgency = (records: CrmRecord[], zone: string) => records.map((record) => ({ record, group: groupOf(record, zone) })).sort((a, b) => a.group - b.group)

// FollowUpQueue lists follow-ups as a review queue, grouped by when each is
// due, under the controls that choose them. Choosing one opens its
// conversation and draft beside the list, or in its place when the queue is
// narrow.
export function FollowUpQueue({ records, focus, onFocus, controls, empty }: { records: CrmRecord[]; focus: number; onFocus: (index: number) => void; controls: ReactNode; empty: ReactNode }) {
  const preparing = useConnections()?.connections.some((c) => c.status === 'active' && c.step === 'FollowUps')
  const zone = useWorkspace()?.timezone ?? 'UTC'
  const queue = byUrgency(records, zone)
  const selected = records[focus]
  return (
    <div className="@container flex min-h-0 flex-1">
      <div className={cn('relative flex min-h-0 min-w-0 flex-col', selected ? 'hidden w-[340px] shrink-0 border-r border-border @4xl:flex @6xl:w-[400px]' : 'flex-1')}>
        <div className="shrink-0 border-b border-border">
          <div className={cn('flex flex-col gap-2.5 px-3 pb-2.5 pt-3', !selected && 'mx-auto max-w-[880px]')}>{controls}</div>
        </div>
        <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
          <div className={cn('flex flex-col gap-0.5 p-2', !selected && 'mx-auto max-w-[880px]')}>
            {empty}
            {groups.map((group, g) => {
              const members = queue.filter((item) => item.group === g)
              return members.length > 0 && (
                <section key={group.name} aria-label={group.name} className="flex flex-col gap-0.5">
                  <h3 className="flex justify-between px-2.5 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-[0.04em]">
                    <span className={group.className}>{group.name}</span>
                    <span className="tabular-nums text-ink-3">{members.length}</span>
                  </h3>
                  <ul className="flex flex-col gap-0.5">
                    {members.map(({ record }) => {
                      const index = records.indexOf(record)
                      return <FollowUp key={record.conversation_id ?? record.id} record={record} zone={zone} index={index} selected={focus === index} onSelect={() => onFocus(index)} />
                    })}
                  </ul>
                </section>
              )
            })}
          </div>
        </div>
        {preparing && <div role="status" className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-full bg-raised px-3 py-1.5 text-[12px] text-ink-2 shadow-md">
          <LoaderCircle aria-hidden="true" className="size-3.5 animate-spin motion-reduce:animate-none" />
          Preparing replies…
        </div>}
      </div>
      {selected && <Conversation key={selected.conversation_id ?? selected.id} record={selected} onClose={() => onFocus(-1)} />}
    </div>
  )
}

const daysSince = (iso: string) => Math.floor((Date.now() - Date.parse(iso)) / oneDay)

// standing says whose move it is, and how long the other side has been
// waiting or the move has been overdue; a dot on the avatar repeats it.
function standing(record: CrmRecord, zone: string) {
  const status = text(record, 'status')
  if (status === 'Done' || status === 'Dismissed') {
    return { label: status, tone: 'text-ink-3', dot: undefined }
  }
  const move = text(record, 'waiting_on')
  const due = text(record, 'action_date')
  const overdue = !!due && isOverdue(due, zone)
  const activity = record.activity
  if (move === 'Them') {
    const since = activity?.last_message?.at ?? activity?.last_at
    const days = since ? daysSince(since) : 0
    return { label: days > 0 ? `${days}d waiting` : 'Waiting', tone: overdue ? 'text-running' : 'text-ink-3', dot: overdue ? 'bg-running' : 'bg-ink-3' }
  }
  const dot = move === 'Us' ? 'bg-primary' : undefined
  if (overdue) {
    const days = daysSince(due.length === 10 ? `${due}T00:00:00Z` : due)
    return { label: days > 0 ? `Overdue ${days}d` : 'Overdue', tone: 'text-danger', dot }
  }
  return move === 'Us' ? { label: 'Your move', tone: 'text-primary', dot } : undefined
}

// lastLine is who said what last: You for the viewer's own mail, otherwise
// the sender's first name.
function lastLine(record: CrmRecord, mine: string[]) {
  const message = record.activity?.last_message
  if (!message) {
    return record.activity?.last_at && `Last contact ${timeAgo(record.activity.last_at)}`
  }
  const sender = mine.includes(message.sender_address ?? '') ? 'You' : (message.sender || message.sender_address || '').split(/[\s@]/)[0]
  return `${sender}: ${message.text}`
}

function FollowUp({ record, zone, index, selected, onSelect }: { record: CrmRecord; zone: string; index: number; selected: boolean; onSelect: () => void }) {
  const me = useWorkspace()?.members?.find((m) => m.is_me)
  const [subject] = refsOf(record)
  const person = valuesOf(record, 'person')[0] as Ref | undefined
  const company = valuesOf(record, 'company')[0] as Ref | undefined
  const state = standing(record, zone)
  const open = text(record, 'status') === 'Open'
  const channel = record.activity?.channel ?? text(record, 'channel').toLowerCase()
  const due = text(record, 'action_date')
  const last = lastLine(record, me ? [me.email, ...(me.addresses ?? [])] : [])
  return (
    <li
      data-row={index}
      aria-current={selected || undefined}
      onClick={onSelect}
      className={cn('group relative flex cursor-default gap-3 rounded-[8px] p-2.5 hover:bg-list-hover', selected && 'bg-list-active hover:bg-list-active')}
    >
      <span className="relative size-8 shrink-0">
        {subject ? <RecordIcon object={subject.plural} name={subject.ref.name ?? ''} photo={subject.ref.photo} size={32} /> : <span aria-hidden="true" className="block size-8 rounded-full bg-list-active" />}
        {state?.dot && <span aria-hidden="true" className={cn('absolute -bottom-0.5 -right-0.5 size-3 rounded-full border-2 border-bg', state.dot)} />}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex min-w-0 items-center justify-between gap-2">
          <div className="flex min-w-0 flex-1 items-center gap-1.5">
            <span className="max-w-[75%] shrink-0 truncate text-[13px] font-semibold text-ink">{subject?.ref.name || recordName(record)}</span>
            {person && company && <>
              <RecordIcon object="companies" name={company.name ?? ''} photo={company.photo} size={14} />
              <span className="min-w-0 truncate text-[12px] text-ink-3">{company.name}</span>
            </>}
          </div>
          {state && <span className={cn('shrink-0 text-[12px]', open && 'transition-opacity duration-150 group-focus-within:opacity-0 group-hover:opacity-0', state.tone)}>{state.label}</span>}
        </div>
        {subject && <p className="truncate text-[12.5px] text-ink-2">{recordName(record)}</p>}
        <div className="flex min-w-0 items-center justify-between gap-2 text-[12px] text-ink-3">
          <div className="flex min-w-0 items-center gap-1.5">
            {channel && <ChannelTag channel={channel} />}
            {last && <span className="truncate">{last}</span>}
          </div>
          {due && <span className="shrink-0" title={text(record, 'action_date_basis') === 'Suggested' ? 'Suggested date' : undefined}><DateLabel value={due} /></span>}
        </div>
      </div>
      <Done record={record} className="absolute right-1.5 top-1.5 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100" />
    </li>
  )
}
