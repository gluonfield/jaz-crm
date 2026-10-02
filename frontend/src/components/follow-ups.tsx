import { Check, X } from 'lucide-react'
import { Fragment, useEffect, useRef, useState } from 'react'
import { Button } from '@jaz/ui/button'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { formatDay } from '@/lib/format'
import { useAction, useTool, useWrite } from '@/lib/queries'
import type { CrmRecord, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { RecordChip } from './controls'

const text = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText).join(', ')
const today = () => new Date().toLocaleDateString('en-CA')

const list = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText)

// Release sends an email draft or approves any other one for its sender, as
// shown; the server refuses a draft that changed since, and anyone but a
// person signed in to the CRM.
function Release({ record, disabled }: { record: CrmRecord; disabled?: boolean }) {
  const send = useAction<{ record_id: string; draft: string; to: string[]; cc: string[] }>('send_draft')
  const state = text(record, 'draft_status')
  const email = text(record, 'channel') === 'Email'
  if (state === 'Sent' || state === 'Sending' || (!email && state === 'Approved')) {
    return <span className="text-[12px] text-ink-3">{state === 'Approved' ? 'Approved · waiting for the sender' : state}</span>
  }
  return (
    <Button variant="primary" size="sm" disabled={disabled || send.isPending} onClick={(e) => {
      e.stopPropagation()
      send.mutate({ record_id: record.id, draft: text(record, 'draft'), to: list(record, 'to'), cc: list(record, 'cc') })
    }}>
      {email ? 'Send' : 'Approve'}
    </Button>
  )
}

function Recipients({ record }: { record: CrmRecord }) {
  const to = text(record, 'to')
  const cc = text(record, 'cc')
  const channel = text(record, 'channel')
  return (
    <span className="min-w-0 truncate text-[12px] text-ink-3">
      {channel === 'Email' ? [to && `To ${to}`, cc && `Cc ${cc}`].filter(Boolean).join(' · ') : channel}
    </span>
  )
}

// FollowUpQueue lists follow-ups as a review queue: what is owed, to whom and
// by when. The focused follow-up opens to its draft beside what we know of
// the person.
export function FollowUpQueue({ records, focus, onFocus }: { records: CrmRecord[]; focus: number; onFocus: (index: number) => void }) {
  return (
    <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
      <ul className="mx-auto max-w-[880px] px-4 py-3">
        {records.map((r, index) => (
          <FollowUp key={r.id} record={r} open={focus === index} index={index} onToggle={() => onFocus(focus === index ? -1 : index)} />
        ))}
      </ul>
    </div>
  )
}

// editDraft starts editing the draft of the follow-up at index, once open.
export const editDraft = (index: number) => document.querySelector<HTMLElement>(`[data-row="${index}"] textarea`)?.focus()

const subjects = { person: 'people', company: 'companies', deal: 'deals' }

function FollowUp({ record, open, index, onToggle }: { record: CrmRecord; open: boolean; index: number; onToggle: () => void }) {
  const write = useWrite(record)
  const row = useRef<HTMLLIElement>(null)
  useEffect(() => {
    if (open) {
      row.current?.scrollIntoView({ block: 'nearest' })
    }
  }, [open])
  const person = valuesOf(record, 'person')[0] as Ref | undefined
  const review = text(record, 'review_on')
  const due = review !== '' && review <= today()
  const waiting = { Us: 'Our move', Them: 'Waiting on them' }[text(record, 'waiting_on')]
  const draft = text(record, 'draft')
  return (
    <li
      ref={row}
      data-row={index}
      aria-expanded={open}
      onClick={onToggle}
      className={cn('group -mx-2 flex cursor-default gap-3 rounded-[var(--radius-control)] px-2 py-2.5 hover:bg-list-hover', open && 'bg-list-hover')}
    >
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex min-w-0 items-baseline gap-2 text-[13px]">
          <span className="truncate font-medium text-ink">{recordName(record)}</span>
          {review && <span className={cn('ml-auto shrink-0 text-[12px] tabular-nums', due ? 'text-ink' : 'text-ink-3')}>{formatDay(review)}</span>}
        </div>
        <div className="flex min-w-0 items-center gap-1.5 text-[12px] text-ink-3">
          {Object.entries(subjects).flatMap(([slug, object]) => (valuesOf(record, slug) as Ref[]).map((v) => <RecordChip key={v.id} object={object} value={v} />))}
          {waiting && <span className="ml-0.5 shrink-0">{waiting}</span>}
        </div>
        {open ? (
          <div className="mt-1.5 flex cursor-auto flex-wrap items-start gap-x-6 gap-y-3" onClick={(e) => e.stopPropagation()}>
            {draft && <Draft key={`${record.id}:${draft}`} record={record} />}
            {person && <Context person={person} />}
          </div>
        ) : draft && (
          <p className="truncate text-[12px] text-ink-3">
            <span className="mr-1.5 text-ink-2">{text(record, 'draft_status') || 'Draft'}</span>
            {draft.replace(/\s+/g, ' ')}
          </p>
        )}
      </div>
      <div className={cn('flex shrink-0 items-start gap-0.5 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100', open && 'opacity-100')}>
        <Button variant="ghost" size="icon-sm" aria-label="Done" title="Done" onClick={(e) => {
          e.stopPropagation()
          write.set('status', 'Done')
        }}>
          <Check />
        </Button>
        <Button variant="ghost" size="icon-sm" aria-label="Dismiss" title="Dismiss" onClick={(e) => {
          e.stopPropagation()
          write.set('status', 'Dismissed')
        }}>
          <X />
        </Button>
      </div>
    </li>
  )
}

// Context is what we know of the person a follow-up concerns, to check and
// fix its draft against: who they are, then dated events as a timeline.
function Context({ person }: { person: Ref }) {
  const record = useTool<CrmRecord>('get_record', { record_id: person.id }).data
  const lines = (record ? text(record, 'context') : '').split('\n').map((line) => line.replace(/^\s*[-*•]\s*/, '').trim()).filter(Boolean)
  if (lines.length === 0) {
    return null
  }
  return (
    <div className="grid min-w-0 flex-[2_1_220px] grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[12px] leading-[18px] text-ink-2">
      {lines.map((line, i) => {
        const [, day, event] = line.match(/^(\d{4}-\d{2}-\d{2}):\s*(.*)$/) ?? []
        return day ? (
          <Fragment key={i}>
            <span className="whitespace-nowrap tabular-nums text-ink-3">{formatDay(day)}</span>
            <span>{event}</span>
          </Fragment>
        ) : (
          <span key={i} className="col-span-2 mb-1">{line}</span>
        )
      })}
    </div>
  )
}

// Draft edits a follow-up's draft in the queue, the one place drafts appear,
// and sends or approves it once the edit is saved.
function Draft({ record }: { record: CrmRecord }) {
  const current = text(record, 'draft')
  const [draft, setDraft] = useState(current)
  const write = useWrite(record)
  const locked = ['Sending', 'Sent'].includes(text(record, 'draft_status'))
  const commit = () => {
    const next = draft.trim()
    if (next !== current && !write.pending) {
      if (next) {
        write.set('draft', next)
      } else {
        write.remove('draft', [])
      }
    }
  }
  return (
    <div className="flex min-w-0 flex-[3_1_380px] flex-col gap-2 rounded-[var(--radius-card)] bg-panel px-3 py-2">
      <textarea
        aria-label="Draft"
        value={draft}
        disabled={write.pending || locked}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          e.stopPropagation()
          if (e.key === 'Escape') {
            setDraft(current)
          } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            e.preventDefault()
            e.currentTarget.blur()
          }
        }}
        className="field-sizing-content block max-h-80 w-full resize-none bg-transparent text-[13px] leading-5 text-ink-2 outline-none focus:text-ink disabled:opacity-70"
      />
      <div className="flex items-center gap-2">
        <Recipients record={record} />
        <span className="ml-auto" />
        <Release record={record} disabled={write.pending || draft.trim() !== current} />
      </div>
    </div>
  )
}
