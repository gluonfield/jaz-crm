import { Check, ChevronDown, ChevronsRight, LoaderCircle, X } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Fragment, useEffect, useRef, useState } from 'react'
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
import { HTMLContent } from './html-content'
import { Release } from './draft-release'

const text = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText).join(', ')
const today = () => new Date().toLocaleDateString('en-CA')

const list = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText)

// FollowUpQueue lists follow-ups as a review queue: what is owed, to whom and
// by when. Choosing one opens its conversation and draft beside the list, or
// in its place when the queue is narrow.
export function FollowUpQueue({ records, focus, onFocus }: { records: CrmRecord[]; focus: number; onFocus: (index: number) => void }) {
  const preparing = useConnections()?.connections.some((c) => c.status === 'active' && c.step === 'FollowUps')
  const selected = records[focus]
  return (
    <div className="@container flex min-h-0 flex-1">
      <div className={cn('relative flex min-h-0 flex-col', selected ? 'hidden w-[320px] shrink-0 border-r border-border @4xl:flex @6xl:w-[380px]' : 'flex-1')}>
        <ul className={cn('scrollbar-quiet min-h-0 w-full flex-1 overflow-y-auto px-4 py-3', !selected && 'mx-auto max-w-[880px]')}>
          {records.map((r, index) => (
            <FollowUp key={r.id} record={r} index={index} selected={focus === index} onSelect={() => onFocus(index)} />
          ))}
        </ul>
        {preparing && <div role="status" className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-full bg-raised px-3 py-1.5 text-[12px] text-ink-2 shadow-md">
          <LoaderCircle aria-hidden="true" className="size-3.5 animate-spin motion-reduce:animate-none" />
          Preparing replies…
        </div>}
      </div>
      {selected && <Conversation key={selected.id} record={selected} onClose={() => onFocus(-1)} />}
    </div>
  )
}

// editDraft starts editing the open follow-up's draft.
export const editDraft = () => document.querySelector<HTMLElement>('[data-conversation] textarea')?.focus()

const subjects = { person: 'people', company: 'companies', deal: 'deals' }
const channels: Record<string, string> = { email: 'Email', linkedin: 'LinkedIn' }

function FollowUp({ record, index, selected, onSelect }: { record: CrmRecord; index: number; selected: boolean; onSelect: () => void }) {
  const draft = text(record, 'draft')
  const review = text(record, 'review_on')
  const who = Object.keys(subjects).flatMap((slug) => (valuesOf(record, slug) as Ref[]).map((v) => v.name))
  const waiting = { Us: 'Our move', Them: 'Waiting on them' }[text(record, 'waiting_on')]
  return (
    <li
      data-row={index}
      aria-current={selected || undefined}
      onClick={onSelect}
      className={cn(
        'group relative -mx-2 flex cursor-default flex-col gap-0.5 rounded-[var(--radius-card)] px-2 py-2.5 text-[12px] text-ink-3 hover:bg-list-hover',
        'before:absolute before:inset-x-2 before:top-0 before:h-px before:bg-border first:before:hidden hover:before:opacity-0 [&:hover+li]:before:opacity-0',
        selected && 'bg-list-hover before:opacity-0 [&+li]:before:opacity-0',
      )}
    >
      <div className="flex min-w-0 items-baseline gap-2">
        <span className="truncate text-[13px] font-medium text-ink">{recordName(record)}</span>
        {review && <span className={cn('ml-auto shrink-0 tabular-nums transition-opacity duration-150 group-focus-within:opacity-0 group-hover:opacity-0', review <= today() && 'text-ink')}>{formatDay(review)}</span>}
      </div>
      <p className="truncate">{[...who, waiting].filter(Boolean).join(' · ')}</p>
      {draft && <p className="truncate">
        <span className="mr-1.5 text-ink-2">{text(record, 'draft_status') || 'Draft'}</span>
        {draft.replace(/\s+/g, ' ')}
      </p>}
      <Actions record={record} className="absolute right-1 top-1.5 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100" />
    </li>
  )
}

// Summary is the open follow-up's title and due date over whom it concerns and
// whose move it is.
function Summary({ record }: { record: CrmRecord }) {
  const review = text(record, 'review_on')
  const waiting = { Us: 'Our move', Them: 'Waiting on them' }[text(record, 'waiting_on')]
  return (
    <>
      <div className="flex min-w-0 items-baseline gap-2">
        <h2 className="min-w-0 text-[15px] font-medium leading-snug text-ink">{recordName(record)}</h2>
        {review && <span className={cn('ml-auto shrink-0 text-[12px] tabular-nums', review <= today() ? 'text-ink' : 'text-ink-3')}>{formatDay(review)}</span>}
      </div>
      <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-[12px] text-ink-3">
        {Object.entries(subjects).flatMap(([slug, object]) => (valuesOf(record, slug) as Ref[]).map((v) => <RecordChip key={v.id} object={object} value={v} />))}
        {waiting && <span className="ml-0.5 shrink-0">{waiting}</span>}
      </div>
    </>
  )
}

function Actions({ record, className }: { record: CrmRecord; className?: string }) {
  const write = useWrite(record)
  return (
    <div className={cn('flex shrink-0 items-start gap-0.5', className)}>
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
  )
}

// Conversation is the open follow-up: what we know of the person, the
// conversation it answers and its draft reply.
function Conversation({ record, onClose }: { record: CrmRecord; onClose: () => void }) {
  const person = valuesOf(record, 'person')[0] as Ref | undefined
  const context = useTool<CrmRecord>('get_record', { record_id: person?.id ?? '' }, { enabled: !!person })
  const conversations = useTool<{ interactions: Interaction[] }>('list_interactions', { record_id: record.id, kinds: ['message'], limit: 1 }, {
    refetchInterval: (query) => (query.state.data?.interactions[0]?.drafting?.state === 'drafting' ? 3000 : false),
  })
  const conversation = conversations.data?.interactions[0]
  const drafting = conversation?.drafting
  const draftingState = drafting?.state
  const draftingStartedAt = drafting?.started_at
  const client = useQueryClient()
  useEffect(() => {
    if (draftingState && draftingState !== 'drafting') {
      void client.invalidateQueries({ queryKey: ['search_records'] })
    }
  }, [draftingState, draftingStartedAt, client])
  const thread = useTool<Interaction>('get_interaction', { interaction_id: conversation?.id ?? '' }, { enabled: !!conversation })
  const channel = text(record, 'channel') || channels[conversation?.channel ?? ''] || ''
  const sender = useTool<DraftSender>('get_draft_sender', { record_id: record.id }, { enabled: channel === 'Email' })
  // A query without data reads as pending again on every refetch, even after
  // it failed, so loading means not yet answered.
  const loading = !conversations.isFetched || (!!conversation && !thread.isFetched)
  const messages = thread.data?.messages ?? (conversation?.last_message ? [conversation.last_message] : [])
  const body = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!loading && body.current) {
      body.current.scrollTop = body.current.scrollHeight
    }
  }, [loading])
  return (
    <section data-conversation aria-label={recordName(record)} className="flex min-w-0 flex-1 flex-col">
      <header className="shrink-0 border-b border-border px-5 pb-4 pt-2">
        <div className="-mx-2 mb-1 flex items-center">
          <Button variant="ghost" size="icon-sm" aria-label="Close" title="Close" onClick={onClose}>
            <ChevronsRight />
          </Button>
          <Actions record={record} className="ml-auto" />
        </div>
        <Summary record={record} />
        {context.data && <Context record={context.data} />}
      </header>
      <div ref={body} className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto px-5 py-4">
        {loading ? (
          <div role="status" className="flex h-28 items-center justify-center gap-2 text-[12px] text-ink-3">
            <LoaderCircle aria-hidden="true" className="size-4 animate-spin motion-reduce:animate-none" />
            Loading conversation…
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            {(conversations.error || thread.error) && <p role="alert" className="text-[12px] text-ink-3">{(conversations.error || thread.error)?.message}</p>}
            {conversation && (messages.length > 0
              ? <MessageThread key={conversation.id} interaction={thread.data ?? conversation} messages={messages} initialVisible={4} />
              : <p className="text-[12px] text-ink-3">Message text is not available yet.</p>)}
            <Draft key={text(record, 'draft_status') === 'Sent' ? 'sent' : 'draft'} record={record} channel={channel} sender={sender.data} error={sender.error?.message} drafting={drafting} />
          </div>
        )}
      </div>
    </section>
  )
}

// Context is what we know of the person: who they are, with the dated
// history one step away.
function Context({ record }: { record: CrmRecord }) {
  const [open, setOpen] = useState(false)
  const lines = text(record, 'context').split('\n').map((line) => line.replace(/^\s*[-*•]\s*/, '').trim()).filter(Boolean)
  const events = lines.flatMap((line) => {
    const [, day, event] = line.match(/^(\d{4}-\d{2}-\d{2}):\s*(.*)$/) ?? []
    return day ? [{ day, event }] : []
  })
  const about = lines.filter((line) => !/^\d{4}-\d{2}-\d{2}:/.test(line))
  if (lines.length === 0) {
    return null
  }
  return (
    <section aria-label={`${recordName(record)} context`} className="mt-3 rounded-[var(--radius-control)] bg-list-hover px-3 py-2 text-[12px] leading-[18px] text-ink-2">
      {about.map((line, i) => <p key={i}>{line}</p>)}
      {events.length > 0 && (
        <>
          <button type="button" aria-expanded={open} onClick={() => setOpen(!open)} className="-mx-1 mt-1 inline-flex items-center gap-1 rounded-[var(--radius-control)] px-1 text-ink-3 outline-none hover:text-ink-2 focus-visible:ring-2 focus-visible:ring-primary">
            History <span className="tabular-nums">{events.length}</span>
            <ChevronDown aria-hidden="true" className={cn('size-3.5 transition-transform duration-150 motion-reduce:transition-none', open && 'rotate-180')} />
          </button>
          {open && <div className="mt-1.5 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5">
            {events.map((e, i) => (
              <Fragment key={i}>
                <span className="whitespace-nowrap tabular-nums text-ink-3">{formatDay(e.day)}</span>
                <span>{e.event}</span>
              </Fragment>
            ))}
          </div>}
        </>
      )}
    </section>
  )
}

function Address({ label, value }: { label: string; value?: string }) {
  return value ? (
    <span className="min-w-0 [overflow-wrap:anywhere]" title={value}>
      {label} <span className="text-ink-2">{value}</span>
    </span>
  ) : null
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
      {sender?.signature && <div className="cursor-default border-t border-border pt-2"><HTMLContent html={sender.signature} className="[&_img]:max-h-16" /></div>}
      <div className="flex justify-end">
        <Release record={record} draft={draft} channel={channel} sender={sender} beforeSend={commit} disabled={!!error} />
      </div>
    </div>
  )
}
