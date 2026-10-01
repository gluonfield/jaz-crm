import { Check, X } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { formatDay } from '@/lib/format'
import { useAction, useWrite } from '@/lib/queries'
import type { CrmRecord, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { RecordIcon } from './icons'

const text = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText).join(', ')
const today = () => new Date().toLocaleDateString('en-CA')

// Release sends an email draft or approves any other one for its sender; the
// server allows it only to a person in the app.
function Release({ record }: { record: CrmRecord }) {
  const send = useAction<{ record_id: string }>('send_draft')
  const state = text(record, 'draft_status')
  const email = text(record, 'channel') === 'Email'
  if (state === 'Sent' || state === 'Sending' || (!email && state === 'Approved')) {
    return <span className="text-[12px] text-ink-3">{state === 'Approved' ? 'Approved · waiting for the sender' : state}</span>
  }
  return (
    <Button variant="primary" size="sm" disabled={send.isPending} onClick={(e) => {
      e.stopPropagation()
      send.mutate({ record_id: record.id })
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

// FollowUpQueue lists follow-ups as a review queue: who, what is owed and by
// when, with any draft ready to send or approve.
export function FollowUpQueue({ records, focus, onOpen }: { records: CrmRecord[]; focus: number; onOpen: (index: number) => void }) {
  return (
    <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
      <ul className="mx-auto max-w-[880px] px-4 py-3">
        {records.map((r, index) => (
          <FollowUp key={r.id} record={r} focused={focus === index} index={index} onOpen={() => onOpen(index)} />
        ))}
      </ul>
    </div>
  )
}

function FollowUp({ record, focused, index, onOpen }: { record: CrmRecord; focused: boolean; index: number; onOpen: () => void }) {
  const write = useWrite(record)
  const person = valuesOf(record, 'person')[0] as Ref | undefined
  const who = [person?.name, text(record, 'company'), text(record, 'deal')].filter(Boolean).join(' · ')
  const review = text(record, 'review_on')
  const due = review !== '' && review <= today()
  const waiting = { Us: 'Our move', Them: 'Waiting on them' }[text(record, 'waiting_on')]
  const draft = text(record, 'draft')
  return (
    <li
      data-row={index}
      onClick={onOpen}
      className={cn('group -mx-2 flex cursor-default gap-3 rounded-[var(--radius-control)] px-2 py-2.5 hover:bg-list-hover', focused && 'bg-list-hover')}
    >
      <RecordIcon object="people" name={person?.name ?? recordName(record)} photo={person?.photo} size={26} className="mt-px" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex min-w-0 items-baseline gap-2 text-[13px]">
          <span className="truncate font-medium text-ink">{recordName(record)}</span>
          {review && <span className={cn('ml-auto shrink-0 text-[12px] tabular-nums', due ? 'text-ink' : 'text-ink-3')}>{formatDay(review)}</span>}
        </div>
        {(who || waiting) && <div className="truncate text-[12px] text-ink-3">{[who, waiting].filter(Boolean).join(' · ')}</div>}
        {draft && (
          <div className="flex flex-col gap-2 rounded-[var(--radius-card)] bg-panel px-3 py-2">
            <p className="line-clamp-4 whitespace-pre-wrap text-[13px] leading-5 text-ink-2">{draft}</p>
            <div className="flex items-center gap-2">
              <Recipients record={record} />
              <span className="ml-auto" />
              <Release record={record} />
            </div>
          </div>
        )}
      </div>
      <div className="flex shrink-0 items-start gap-0.5 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100">
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

// DraftPanel edits a follow-up's draft on its page and sends or approves it.
export function DraftPanel({ record }: { record: CrmRecord }) {
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
    <section aria-label="Draft" className="mt-6">
      <div className="mb-1.5 flex items-center gap-2">
        <h2 className="text-[12px] font-medium text-ink-3">Draft</h2>
        {current && <Recipients record={record} />}
        {current && (
          <span className="ml-auto">
            <Release record={record} />
          </span>
        )}
      </div>
      <textarea
        aria-label="Draft"
        value={draft}
        disabled={write.pending || locked}
        rows={4}
        placeholder="Write a reply…"
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            setDraft(current)
          } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            e.preventDefault()
            e.currentTarget.blur()
          }
        }}
        className="field-sizing-content block max-h-96 min-h-24 w-full resize-y rounded-[var(--radius-control)] border border-transparent bg-transparent px-2 py-2 text-[13px] leading-5 text-ink outline-none placeholder:text-ink-3 hover:bg-list-hover focus:border-border focus:bg-bg disabled:opacity-70"
      />
    </section>
  )
}
