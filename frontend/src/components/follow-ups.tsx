import { Check, X } from 'lucide-react'
import { Fragment, type ReactNode, useEffect, useRef, useState } from 'react'
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

// Release sends an email draft from the mailbox shown, or approves any other
// one for its sender; the server refuses a draft that changed since, and
// anyone but a person signed in to the CRM.
function Release({ record, from, disabled }: { record: CrmRecord; from?: string; disabled?: boolean }) {
  const send = useAction<{ record_id: string; draft: string; from?: string; to: string[]; cc: string[] }>('send_draft')
  const state = text(record, 'draft_status')
  const email = text(record, 'channel') === 'Email'
  if (state === 'Sent' || state === 'Sending' || (!email && state === 'Approved')) {
    return <span className="text-[12px] text-ink-3">{state === 'Approved' ? 'Approved · waiting for the sender' : state}</span>
  }
  return (
    <Button variant="primary" size="sm" disabled={disabled || send.isPending || (email && !from)} onClick={() => {
      send.mutate({ record_id: record.id, draft: text(record, 'draft'), from, to: list(record, 'to'), cc: list(record, 'cc') })
    }}>
      {email ? 'Send' : 'Approve'}
    </Button>
  )
}

// FollowUpQueue lists follow-ups as a review queue: what is owed, to whom and
// by when. The focused follow-up opens to its draft and what we know of the
// person.
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
  const email = text(record, 'channel') === 'Email'
  const sender = useTool<{ from: string }>('get_draft_sender', { record_id: record.id }, { enabled: open && email && draft !== '' })
  return (
    <li
      ref={row}
      data-row={index}
      aria-expanded={open}
      onClick={onToggle}
      className={cn('group -mx-2 flex cursor-default gap-3 rounded-[var(--radius-control)] px-2 py-2.5 hover:bg-list-hover', open && 'bg-list-hover')}
    >
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex min-w-0 items-baseline gap-2 text-[13px]">
          <span className="truncate font-medium text-ink">{recordName(record)}</span>
          {review && <span className={cn('ml-auto shrink-0 text-[12px] tabular-nums', due ? 'text-ink' : 'text-ink-3')}>{formatDay(review)}</span>}
        </div>
        <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-[12px] text-ink-3">
          {Object.entries(subjects).flatMap(([slug, object]) => (valuesOf(record, slug) as Ref[]).map((v) => <RecordChip key={v.id} object={object} value={v} />))}
          {waiting && <span className="ml-0.5 shrink-0">{waiting}</span>}
          {open && draft && (
            <span className="ml-auto flex min-w-0 items-center gap-3 transition-opacity duration-150 starting:opacity-0">
              {email ? (
                <>
                  {sender.error ? <span className="truncate text-ink-2">{sender.error.message}</span> : <Address label="From" value={sender.data?.from} />}
                  <Address label="To" value={text(record, 'to')} />
                  <Address label="Cc" value={text(record, 'cc')} />
                </>
              ) : text(record, 'channel')}
            </span>
          )}
        </div>
        <Reveal open={!open && draft !== ''}>
          <p className="truncate pt-1.5 text-[12px] text-ink-3">
            <span className="mr-1.5 text-ink-2">{text(record, 'draft_status') || 'Draft'}</span>
            {draft.replace(/\s+/g, ' ')}
          </p>
        </Reveal>
        <Reveal open={open} onOpened={() => row.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })}>
          <div className="flex cursor-auto flex-col gap-3 pt-3" onClick={(e) => e.stopPropagation()}>
            {draft && <Draft key={`${record.id}:${draft}`} record={record} from={sender.data?.from} />}
            {person && <Context person={person} load={open} />}
          </div>
        </Reveal>
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

// Reveal grows its content open and shut, keeping it out of reach while shut.
function Reveal({ open, onOpened, children }: { open: boolean; onOpened?: () => void; children: ReactNode }) {
  return (
    <div
      inert={!open}
      onTransitionEnd={(e) => open && e.target === e.currentTarget && e.propertyName === 'grid-template-rows' && onOpened?.()}
      className={cn('grid transition-[grid-template-rows,opacity] duration-150 ease-[cubic-bezier(0.2,0,0,1)] motion-reduce:transition-none', open ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0')}
    >
      <div className="min-h-0 overflow-hidden">{children}</div>
    </div>
  )
}

function Address({ label, value }: { label: string; value?: string }) {
  return value ? (
    <span className="min-w-0 truncate" title={value}>
      {label} <span className="text-ink-2">{value}</span>
    </span>
  ) : null
}

// Context is what we know of the person a follow-up concerns, to check and
// fix its draft against: who they are, then dated events as a timeline.
function Context({ person, load }: { person: Ref; load: boolean }) {
  const record = useTool<CrmRecord>('get_record', { record_id: person.id }, { enabled: load }).data
  const lines = (record ? text(record, 'context') : '').split('\n').map((line) => line.replace(/^\s*[-*•]\s*/, '').trim()).filter(Boolean)
  if (lines.length === 0) {
    return null
  }
  return (
    <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 px-3 text-[12px] leading-[18px] text-ink-2">
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
function Draft({ record, from }: { record: CrmRecord; from?: string }) {
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
    <div className="flex flex-col gap-2 rounded-[var(--radius-card)] bg-panel px-3 py-2.5">
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
        className="field-sizing-content block max-h-96 min-h-24 w-full resize-none bg-transparent text-[13px] leading-5 text-ink-2 outline-none focus:text-ink disabled:opacity-70"
      />
      <div className="flex justify-end">
        <Release record={record} from={from} disabled={write.pending || draft.trim() !== current} />
      </div>
    </div>
  )
}
