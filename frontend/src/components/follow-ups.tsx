import { LoaderCircle } from 'lucide-react'
import type { ReactNode } from 'react'
import { recordName, textOf, valuesOf } from '@/lib/crm'
import { dueLabel, formatActionDate } from '@/lib/dates'
import { type Standing, standingOf } from '@/lib/follow-ups'
import { useWorkspace } from '@/lib/queries'
import { useConnections } from '@/lib/sync'
import type { CrmRecord, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Conversation, Done, refsOf } from './follow-up'
import { RecordIcon } from './icons'

const groups = [
  { name: 'To do', className: 'text-ink' },
  { name: 'Waiting on them', className: 'text-ink-3' },
  { name: 'Done', className: 'text-ink-3' },
  { name: 'Dismissed', className: 'text-ink-3' },
]

const groupOf: Record<Standing, number> = { reply: 0, todo: 0, chase: 0, waiting: 1, done: 2, dismissed: 3 }

// byUrgency orders follow-ups by whose move each is, keeping their order
// within each group.
export const byUrgency = (records: CrmRecord[], zone: string) => records.map((record) => {
  const standing = standingOf(record, zone)
  return { record, standing, group: groupOf[standing] }
}).sort((a, b) => a.group - b.group)

// FollowUpQueue lists follow-ups as a review queue, grouped by whose move
// each is, under the controls that choose them. Choosing one opens its
// conversation and draft beside the list, or in its place when the queue is
// narrow.
export function FollowUpQueue({ records, focus, onFocus, controls, empty }: { records: CrmRecord[]; focus: number; onFocus: (index: number) => void; controls: ReactNode; empty: ReactNode }) {
  const preparing = useConnections()?.connections.some((c) => c.status === 'active' && c.step === 'FollowUps')
  const zone = useWorkspace()?.timezone ?? 'UTC'
  const queue = byUrgency(records, zone)
  const selected = records[focus]
  return (
    <div className="@container flex min-h-0 flex-1">
      <div className={cn('relative flex min-h-0 min-w-0 flex-col', selected ? 'hidden w-[340px] shrink-0 border-r border-border @4xl:flex' : 'flex-1')}>
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
                  <h3 data-flip={`group:${group.name}`} className="flex justify-between px-2.5 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-[0.04em]">
                    <span className={group.className}>{group.name}</span>
                    <span className="tabular-nums text-ink-3">{members.length}</span>
                  </h3>
                  <ul className="flex flex-col gap-0.5">
                    {members.map((item) => {
                      const index = records.indexOf(item.record)
                      return <FollowUp key={item.record.conversation_id ?? item.record.id} record={item.record} standing={item.standing} zone={zone} index={index} selected={focus === index} onSelect={() => onFocus(index)} />
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

// tags say what a to-do asks of you when the row alone cannot.
const tags: Partial<Record<Standing, string>> = { reply: 'Reply', chase: 'Chase' }

function FollowUp({ record, standing, zone, index, selected, onSelect }: { record: CrmRecord; standing: Standing; zone: string; index: number; selected: boolean; onSelect: () => void }) {
  const [subject] = refsOf(record)
  const person = valuesOf(record, 'person')[0] as Ref | undefined
  const company = valuesOf(record, 'company')[0] as Ref | undefined
  const waiting = standing === 'waiting'
  const open = standing !== 'done' && standing !== 'dismissed'
  const due = textOf(record, 'action_date')
  const when = due && dueLabel(due, zone)
  const tag = tags[standing] && <span className={cn('shrink-0 rounded-[4px] px-[5px] text-[10.5px] font-semibold leading-4', standing === 'reply' ? 'bg-primary-soft text-primary' : 'bg-list-active text-ink-2')}>{tags[standing]}</span>
  return (
    <li
      data-flip={record.conversation_id ?? record.id}
      data-row={index}
      aria-current={selected || undefined}
      onClick={onSelect}
      className={cn('group relative flex cursor-default gap-2.5 rounded-[8px] px-2.5 py-2 hover:bg-list-hover', selected && 'bg-list-active hover:bg-list-active')}
    >
      {subject ? <RecordIcon object={subject.plural} name={subject.ref.name ?? ''} photo={subject.ref.photo} size={26} className={cn(waiting && 'opacity-70')} /> : <span aria-hidden="true" className="block size-[26px] shrink-0 rounded-full bg-list-active" />}
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <div className="flex min-w-0 items-center gap-1.5">
          {!subject && tag}
          <span className={cn('min-w-0 shrink truncate text-[13px] font-medium', waiting ? 'text-ink-2' : 'text-ink')}>{subject?.ref.name || recordName(record)}</span>
          {person && company && <span className="min-w-0 flex-1 truncate text-[12px] text-ink-3">{company.name}</span>}
          {when && <span title={waiting ? `Chase ${formatActionDate(due, zone)}` : undefined} className={cn('ml-auto shrink-0 pl-2 text-[11.5px] tabular-nums', when.late ? 'text-danger' : 'text-ink-3', open && 'transition-opacity duration-150 group-focus-within:opacity-0 group-hover:opacity-0')}>{when.label}</span>}
        </div>
        {subject && <p className="flex min-w-0 items-center gap-1.5 text-[12px] text-ink-3">
          {tag}
          <span className="truncate">{recordName(record)}</span>
        </p>}
      </div>
      {/* On a touch screen, Done is in the opened follow-up rather than an invisible tap target here. */}
      <Done record={record} className="absolute right-1.5 top-1 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100 pointer-coarse:hidden" />
    </li>
  )
}
