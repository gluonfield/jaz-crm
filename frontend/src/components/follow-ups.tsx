import { Check, ChevronsRight, LoaderCircle } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
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
import { MarkdownView } from './editor'
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
const moves: Record<string, string> = { Us: 'Our move', Them: 'Waiting on them' }
const refsOf = (record: CrmRecord) => Object.entries(subjects).flatMap(([object, plural]) => (valuesOf(record, object) as Ref[]).map((ref) => ({ plural, ref })))

function FollowUp({ record, index, selected, onSelect }: { record: CrmRecord; index: number; selected: boolean; onSelect: () => void }) {
  const draft = text(record, 'draft')
  const review = text(record, 'review_on')
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
      <p className="truncate">{[...refsOf(record).map(({ ref }) => ref.name), moves[text(record, 'waiting_on')]].filter(Boolean).join(' · ')}</p>
      {draft && <p className="truncate">
        <span className="mr-1.5 text-ink-2">{text(record, 'draft_status') || 'Draft'}</span>
        {draft.replace(/\s+/g, ' ')}
      </p>}
      <Done record={record} className="absolute right-1 top-1.5 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100" />
    </li>
  )
}

// Summary is the open follow-up's title and due date over whom it concerns and
// whose move it is.
function Summary({ record }: { record: CrmRecord }) {
  const review = text(record, 'review_on')
  const move = moves[text(record, 'waiting_on')]
  return (
    <>
      <div className="flex min-w-0 items-baseline gap-2">
        <h2 className="min-w-0 text-[15px] font-medium leading-snug text-ink">{recordName(record)}</h2>
        {review && <span className={cn('ml-auto shrink-0 text-[12px] tabular-nums', review <= today() ? 'text-ink' : 'text-ink-3')}>{formatDay(review)}</span>}
      </div>
      <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-[12px] text-ink-3">
        {refsOf(record).map(({ plural, ref }) => <RecordChip key={ref.id} object={plural} value={ref} />)}
        {move && <span className="ml-0.5 shrink-0">{move}</span>}
      </div>
    </>
  )
}

function Done({ record, className }: { record: CrmRecord; className?: string }) {
  const write = useWrite(record)
  return (
    <Button variant="ghost" size="icon-sm" aria-label="Done" title="Done" className={className} onClick={(e) => {
      e.stopPropagation()
      write.set('status', 'Done')
    }}>
      <Check />
    </Button>
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
  const error = conversations.error ?? thread.error
  const body = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
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
          <Done record={record} className="ml-auto" />
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
          <>
            {error && <p role="alert" className="mb-4 text-[12px] text-ink-3">{error.message}</p>}
            {conversation && (messages.length > 0
              ? <MessageThread key={conversation.id} interaction={thread.data ?? conversation} messages={messages} initialVisible={4} />
              : <p className="text-[12px] text-ink-3">Message text is not available yet.</p>)}
          </>
        )}
      </div>
      <footer className="shrink-0 px-5 pb-4">
        <Draft record={record} channel={channel} sender={sender.data} error={sender.error?.message} drafting={drafting} />
      </footer>
    </section>
  )
}

// Context is what we know of the person, as its markdown says it.
function Context({ record }: { record: CrmRecord }) {
  const context = text(record, 'context')
  return context ? (
    <section aria-label={`${recordName(record)} context`} className="scrollbar-quiet mt-3 max-h-36 overflow-y-auto rounded-[var(--radius-control)] bg-list-hover px-3 py-2">
      <MarkdownView text={context} />
    </section>
  ) : null
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
    // A draft being sent cannot change; the new text stays here and saves on
    // the next blur once the send settles.
    if (text(record, 'draft_status') === 'Sending' || (!saving.current && next === current)) {
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
  const person = (valuesOf(record, 'person')[0] as Ref | undefined)?.name
  // The AI's state shows while it works or could not help, and beside the
  // draft it wrote; addresses show while a reply is being written.
  const status = drafting && (drafting.state !== 'completed' || current) ? drafting : undefined
  return (
    <div className="group rounded-[var(--radius-card)] bg-list-hover transition-colors duration-150 focus-within:bg-list-active">
      {status && <p role="status" aria-live="polite" className="flex items-start gap-1.5 px-3 pt-2 text-[12px] leading-[18px] text-ink-3">
        {status.state === 'drafting' && <LoaderCircle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 animate-spin motion-reduce:animate-none" />}
        <span>
          <span className={cn('font-medium', status.state === 'failed' ? 'text-danger' : 'text-ink-2')}>{ { drafting: 'Drafting a reply…', completed: 'Reply drafted', failed: 'Couldn’t draft a reply', skipped: 'No reply drafted' }[status.state] }</span>
          {status.reason && <> · {status.reason}</>}
        </span>
      </p>}
      {email && <p className={cn('flex min-w-0 flex-wrap gap-x-3 px-3 text-[12px] leading-[18px] text-ink-3', status ? 'pt-0.5' : 'pt-2', !(draft || error) && 'hidden group-focus-within:flex')}>
        {error ? <span className="text-ink-2">{error}</span> : <Address label="From" value={sender?.from} />}
        <Address label="To" value={sender?.to.join(', ')} />
        <Address label="Cc" value={sender?.cc.join(', ')} />
      </p>}
      <div className="flex items-end gap-2 py-1.5 pl-3 pr-1.5">
        <textarea
          aria-label="Draft"
          rows={1}
          placeholder={person ? `Reply to ${person}…` : 'Write a reply…'}
          value={draft}
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
          className="field-sizing-content block max-h-[40dvh] min-h-7 min-w-0 flex-1 resize-none bg-transparent py-1 text-[13px] leading-5 text-ink-2 outline-none placeholder:text-ink-3 focus:text-ink"
        />
        <Release record={record} draft={draft} channel={channel} sender={sender} beforeSend={commit} disabled={!!error} />
      </div>
    </div>
  )
}
