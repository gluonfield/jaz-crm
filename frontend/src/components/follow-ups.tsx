import { Check, ChevronDown, LoaderCircle, X } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Fragment, type ReactNode, useEffect, useRef, useState } from 'react'
import { Button } from '@jaz/ui/button'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { formatDay } from '@/lib/format'
import { call } from '@/lib/api'
import { toolQuery, useTool, useWrite } from '@/lib/queries'
import { useConnections } from '@/lib/sync'
import type { CrmRecord, DraftSender, Interaction, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { RecordChip } from './controls'
import { MessageThread } from './email-thread'
import { Signature } from './signature'
import { Release } from './draft-release'

const text = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText).join(', ')
const today = () => new Date().toLocaleDateString('en-CA')

const list = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText)

// FollowUpQueue lists follow-ups as a review queue: what is owed, to whom and
// by when. The focused follow-up opens to its draft and what we know of the
// person.
export function FollowUpQueue({ records, focus, onFocus }: { records: CrmRecord[]; focus: number; onFocus: (index: number) => void }) {
  const preparing = useConnections(3000)?.connections.some((c) => c.status === 'active' && c.step === 'FollowUps')
  return (
    <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
      {preparing && <div role="status" className="mx-auto flex max-w-[880px] items-center gap-2 px-4 pt-3 text-[12px] text-ink-2">
        <LoaderCircle aria-hidden="true" className="size-3.5 animate-spin motion-reduce:animate-none" />
        Preparing replies…
      </div>}
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
const channels: Record<string, string> = { email: 'Email', linkedin: 'LinkedIn' }

function FollowUp({ record, open, index, onToggle }: { record: CrmRecord; open: boolean; index: number; onToggle: () => void }) {
  const write = useWrite(record)
  const person = valuesOf(record, 'person')[0] as Ref | undefined
  const review = text(record, 'review_on')
  const due = review !== '' && review <= today()
  const waiting = { Us: 'Our move', Them: 'Waiting on them' }[text(record, 'waiting_on')]
  const draft = text(record, 'draft')
  const context = useTool<CrmRecord>('get_record', { record_id: person?.id ?? '' }, { enabled: open && !!person })
  const conversations = useTool<{ interactions: Interaction[] }>('list_interactions', { record_id: record.id, kinds: ['message'], limit: 1 }, { enabled: open, refetchInterval: open ? 3000 : false })
  const conversation = conversations.data?.interactions[0]
  const drafting = conversation?.drafting
  const draftingState = drafting?.state
  const draftingStartedAt = drafting?.started_at
  const client = useQueryClient()
  useEffect(() => {
    if (open && draftingState && draftingState !== 'drafting') {
      void client.invalidateQueries({ queryKey: ['search_records'] })
    }
  }, [open, draftingState, draftingStartedAt, client])
  const thread = useTool<Interaction>('get_interaction', { interaction_id: conversation?.id ?? '' }, { enabled: open && !!conversation })
  const channel = text(record, 'channel') || channels[conversation?.channel ?? ''] || ''
  const email = channel === 'Email'
  const sender = useTool<DraftSender>('get_draft_sender', { record_id: record.id }, { enabled: open && email })
  const loading = context.isLoading || conversations.isLoading || thread.isLoading || sender.isLoading
  const messages = thread.data?.messages ?? (conversation?.last_message ? [conversation.last_message] : [])
  return (
    <li
      data-row={index}
      aria-expanded={open}
      onClick={onToggle}
      className={cn(
        'group relative -mx-2 flex cursor-default gap-3 rounded-[var(--radius-card)] px-2 py-3 hover:bg-list-hover',
        'before:absolute before:inset-x-2 before:top-0 before:h-px before:bg-border first:before:hidden hover:before:opacity-0 [&:hover+li]:before:opacity-0',
        open && 'bg-list-hover before:opacity-0 [&+li]:before:opacity-0',
      )}
    >
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex min-w-0 items-baseline gap-2 text-[13px]">
          <span className="truncate font-medium text-ink">{recordName(record)}</span>
          {review && <span className={cn('ml-auto shrink-0 text-[12px] tabular-nums', due ? 'text-ink' : 'text-ink-3')}>{formatDay(review)}</span>}
        </div>
        <div className="mt-1 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-[12px] text-ink-3">
          {Object.entries(subjects).flatMap(([slug, object]) => (valuesOf(record, slug) as Ref[]).map((v) => <RecordChip key={v.id} object={object} value={v} />))}
          {waiting && <span className="ml-0.5 shrink-0">{waiting}</span>}
        </div>
        <Reveal open={!open && draft !== ''}>
          <p className="truncate pt-1 text-[12px] text-ink-3">
            <span className="mr-1.5 text-ink-2">{text(record, 'draft_status') || 'Draft'}</span>
            {draft.replace(/\s+/g, ' ')}
          </p>
        </Reveal>
        <Reveal open={open && loading}>
          <div role="status" className="flex h-28 items-center justify-center gap-2 text-[12px] text-ink-3">
            <LoaderCircle aria-hidden="true" className="size-4 animate-spin motion-reduce:animate-none" />
            Loading conversation…
          </div>
        </Reveal>
        <Reveal open={open && !loading}>
          <div id={`follow-up-${record.id}`} className="flex cursor-auto flex-col gap-4 pt-4" onClick={(e) => e.stopPropagation()}>
            {context.error && <p role="alert" className="text-[12px] text-ink-3">{context.error.message}</p>}
            {context.data && <Context record={context.data} />}
            {(conversations.error || thread.error) && <p role="alert" className="text-[12px] text-ink-3">{(conversations.error || thread.error)?.message}</p>}
            {conversation && (messages.length > 0
              ? <MessageThread key={conversation.id} interaction={thread.data ?? conversation} messages={messages} initialVisible={1} />
              : <p className="text-[12px] text-ink-3">Message text is not available yet.</p>)}
            <Draft record={record} channel={channel} sender={sender.data} error={sender.error?.message} drafting={drafting} />
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
function Reveal({ open, children }: { open: boolean; children: ReactNode }) {
  return (
    <div
      inert={!open}
      aria-hidden={!open}
      className={cn('grid transition-[grid-template-rows,opacity] duration-150 ease-[cubic-bezier(0.2,0,0,1)] motion-reduce:transition-none', open ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0')}
    >
      <div className="min-h-0 overflow-hidden">{children}</div>
    </div>
  )
}

function Address({ label, value }: { label: string; value?: string }) {
  return value ? (
    <span className="min-w-0 [overflow-wrap:anywhere]" title={value}>
      {label} <span className="text-ink-2">{value}</span>
    </span>
  ) : null
}

function Context({ record }: { record: CrmRecord }) {
  const [expanded, setExpanded] = useState(true)
  const lines = text(record, 'context').split('\n').map((line) => line.replace(/^\s*[-*•]\s*/, '').trim()).filter(Boolean)
  if (lines.length === 0) {
    return null
  }
  return (
    <section className="rounded-[var(--radius-control)] bg-list-hover px-3 py-2.5">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-[11px] font-medium uppercase tracking-[0.08em] text-ink-3">Person context</h3>
        <Button variant="ghost" size="icon-sm" aria-label={expanded ? 'Hide person context' : 'Show person context'} aria-expanded={expanded} onClick={() => setExpanded(!expanded)} className="-my-1 -mr-1">
          <ChevronDown aria-hidden="true" className={cn('transition-transform duration-150 motion-reduce:transition-none', expanded && 'rotate-180')} />
        </Button>
      </div>
      <Reveal open={expanded}>
        <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 pt-2 text-[12px] leading-[18px] text-ink-2">
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
      </Reveal>
    </section>
  )
}

function Draft({ record, channel, sender, error, drafting }: { record: CrmRecord; channel: string; sender?: DraftSender; error?: string; drafting?: Interaction['drafting'] }) {
  const current = text(record, 'draft')
  const [edited, setEdited] = useState<string | null>(null)
  const saving = useRef<{ text: string; promise: Promise<unknown> } | null>(null)
  const draft = edited ?? current
  const client = useQueryClient()
  const locked = ['Sending', 'Sent'].includes(text(record, 'draft_status'))
  const email = channel === 'Email'
  const to = list(record, 'to')
  const write = useMutation({
    mutationKey: ['save_draft'],
    scope: { id: `draft:${record.id}` },
    mutationFn: async (next: string) => {
      let replyChannel = channel
      let recipients = { to, cc: list(record, 'cc') }
      if (next && !replyChannel) {
        const source = await client.fetchQuery(toolQuery<{ interactions: Interaction[] }>('list_interactions', { record_id: record.id, kinds: ['message'], limit: 1 }))
        replyChannel = channels[source.interactions[0]?.channel ?? ''] ?? ''
      }
      if (next && replyChannel === 'Email' && to.length === 0 && !error) {
        const defaults = sender ?? await client.fetchQuery<DraftSender>(toolQuery<DraftSender>('get_draft_sender', { record_id: record.id }))
        recipients = { to: defaults.to, cc: defaults.cc }
      }
      return call('save_draft', { record_id: record.id, draft: next, channel: replyChannel, ...recipients })
    },
  })
  const commit = (next = draft.trim()): Promise<unknown> => {
    if (saving.current?.text === next) {
      return saving.current.promise
    }
    if (locked || (!saving.current && next === current)) {
      return Promise.resolve()
    }
    const promise = write.mutateAsync(next).then(() => {
      setEdited((latest) => latest?.trim() === next ? null : latest)
    }).finally(() => {
      if (saving.current?.promise === promise) {
        saving.current = null
      }
    })
    saving.current = { text: next, promise }
    return promise
  }
  return (
    <div className="flex flex-col gap-2 rounded-[var(--radius-control)] bg-bg px-3 py-2.5">
      <div className="flex min-w-0 flex-wrap gap-x-3 gap-y-1 text-[12px] text-ink-3">
        <h3 className="font-medium text-ink-2">{channel ? `${channel} reply` : 'Reply'}</h3>
        {email && <>
          {error ? <span className="text-ink-2">{error}</span> : <Address label="From" value={sender?.from} />}
          <Address label="To" value={sender?.to.join(', ')} />
          <Address label="Cc" value={sender?.cc.join(', ')} />
        </>}
      </div>
      {drafting && <div role="status" aria-live="polite" className="flex items-start gap-1.5 text-[12px] leading-[18px] text-ink-3">
        {drafting.state === 'drafting' && <LoaderCircle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 animate-spin motion-reduce:animate-none" />}
        <span>
          <span className={cn('font-medium', drafting.state === 'failed' ? 'text-danger' : 'text-ink-2')}>{ { drafting: 'Drafting…', completed: 'Completed', failed: 'Failed', skipped: 'Skipped' }[drafting.state] }</span>
          {drafting.reason && <> · {drafting.reason}</>}
        </span>
      </div>}
      <textarea
        aria-label="Draft"
        placeholder="Write a reply…"
        value={draft}
        disabled={locked}
        onChange={(e) => setEdited(e.target.value)}
        onBlur={() => void commit().catch(() => {})}
        onKeyDown={(e) => {
          e.stopPropagation()
          if (e.key === 'Escape') {
            setEdited(null)
          } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            e.preventDefault()
            e.currentTarget.blur()
          }
        }}
        className="field-sizing-content block max-h-96 min-h-24 w-full resize-none bg-transparent text-[13px] leading-5 text-ink-2 outline-none focus:text-ink disabled:opacity-70"
      />
      {sender?.signature && <div className="cursor-default border-t border-border pt-2"><Signature html={sender.signature} /></div>}
      <div className="flex justify-end">
        <Release record={record} draft={draft} channel={channel} sender={sender} beforeSend={commit} disabled={!!error} />
      </div>
    </div>
  )
}
