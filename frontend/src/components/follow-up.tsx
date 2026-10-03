import { ArrowLeft, Check, LoaderCircle } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { Button } from '@jaz/ui/button'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { call } from '@/lib/api'
import { isOverdue } from '@/lib/dates'
import { toolQuery, useTool, useWorkspace, useWrite } from '@/lib/queries'
import { readItem, writeItem } from '@/lib/storage'
import type { CrmRecord, DraftSender, Interaction, Party, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { RecordChip } from './controls'
import { DateField, DateLabel } from './date-field'
import { Divider, MessageThread } from './email-thread'
import { MarkdownView } from './editor'
import { ChannelTag, RecordIcon } from './icons'
import { ContextEditor } from './person-context'
import { Release } from './draft-release'

export const text = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText).join(', ')

const list = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText)

const subjects = { person: 'people', company: 'companies', deal: 'deals' }
const channels: Record<string, string> = { email: 'Email', linkedin: 'LinkedIn' }

// refsOf lists whom a follow-up concerns, its person first.
export const refsOf = (record: CrmRecord) => Object.entries(subjects).flatMap(([object, plural]) => (valuesOf(record, object) as Ref[]).map((ref) => ({ plural, ref })))

// editDraft starts editing the open follow-up's draft.
export const editDraft = () => document.querySelector<HTMLElement>('[data-conversation] textarea[aria-label="Draft"]')?.focus()

export function Done({ record, labelled = false, className }: { record: CrmRecord; labelled?: boolean; className?: string }) {
  const write = useWrite(record)
  return (
    <Button variant="ghost" size={labelled ? 'default' : 'icon-sm'} aria-label="Done" title="Done" className={className} onClick={(e) => {
      e.stopPropagation()
      write.set('status', 'Done')
    }}>
      <Check />
      {labelled && 'Done'}
    </Button>
  )
}

// Conversation is the open follow-up: whom it is with and when it is due,
// what we know of them, the conversation it answers and its draft reply.
export function Conversation({ record, onClose }: { record: CrmRecord; onClose: () => void }) {
  const [subject] = refsOf(record)
  const person = valuesOf(record, 'person')[0] as Ref | undefined
  const company = valuesOf(record, 'company')[0] as Ref | undefined
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
  const due = text(record, 'action_date')
  const zone = useWorkspace()?.timezone ?? 'UTC'
  const body = useStickToBottom()
  return (
    <section data-conversation aria-label={recordName(record)} className="flex min-w-0 flex-1 flex-col">
      <header className="flex shrink-0 items-center gap-3 border-b border-border py-3.5 pl-6 pr-4">
        <Button variant="ghost" size="icon-sm" aria-label="Back" title="Back" className="-ml-2 @4xl:hidden" onClick={onClose}>
          <ArrowLeft />
        </Button>
        {subject && <RecordIcon object={subject.plural} name={subject.ref.name ?? ''} photo={subject.ref.photo} size={36} />}
        <div className="flex min-w-0 flex-col gap-[3px]">
          <div className="flex min-w-0 items-center gap-2">
            {subject
              ? <Link to="/r/$recordId" params={{ recordId: subject.ref.id }} className="truncate text-[15px] font-semibold text-ink outline-none hover:underline focus-visible:underline">{subject.ref.name || 'Unnamed'}</Link>
              : <h2 className="truncate text-[15px] font-semibold text-ink">{recordName(record)}</h2>}
            {person && company && <>
              <span aria-hidden="true" className="text-ink-3">·</span>
              <RecordChip object="companies" value={company} />
            </>}
          </div>
          {(channel || conversation) && <p className="flex min-w-0 items-center gap-1.5 text-[12px] text-ink-3">
            {channel && <ChannelTag channel={channel.toLowerCase()} />}
            <span className="truncate">{conversation?.title}</span>
          </p>}
        </div>
        <div className="ml-auto flex shrink-0 items-center gap-1.5">
          <DateField record={record} prefix="Action" />
          <Done record={record} labelled className="bg-list-hover hover:bg-list-active" />
        </div>
      </header>
      {context.data && <Context record={context.data} />}
      <div ref={body} className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto px-6 py-5 @4xl:px-8">
        <div className="flex flex-col gap-3.5">
          {loading ? (
            <div role="status" className="flex h-28 items-center justify-center gap-2 text-[12px] text-ink-3">
              <LoaderCircle aria-hidden="true" className="size-4 animate-spin motion-reduce:animate-none" />
              Loading conversation…
            </div>
          ) : (
            <>
              {error && <p role="alert" className="text-[12px] text-ink-3">{error.message}</p>}
              {conversation && (messages.length > 0
                ? <MessageThread key={conversation.id} interaction={thread.data ?? conversation} messages={messages} events={[{ at: record.created_at, label: <>Follow-up created · {recordName(record)}</> }]} initialVisible={4} />
                : <p className="text-[12px] text-ink-3">Message text is not available yet.</p>)}
              {due && <Divider>{isOverdue(due, zone) ? <span className="text-danger">Overdue</span> : 'Next'} · <DateLabel value={due} /></Divider>}
            </>
          )}
        </div>
      </div>
      <footer className="shrink-0 px-6 pb-5 pt-2">
        <Draft record={record} channel={channel} sender={sender.data} error={sender.error?.message} drafting={drafting} participants={(thread.data ?? conversation)?.participants ?? []} />
      </footer>
    </section>
  )
}

// useStickToBottom keeps a conversation at its latest message, as chat apps
// do, while its content or the reply box below changes size, until someone
// scrolls up to read.
function useStickToBottom() {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const element = ref.current!
    let pinned = true
    const follow = () => {
      if (pinned) {
        element.scrollTop = element.scrollHeight
      }
    }
    const track = () => {
      pinned = element.scrollHeight - element.scrollTop - element.clientHeight < 32
    }
    const observer = new ResizeObserver(follow)
    observer.observe(element)
    observer.observe(element.firstElementChild!)
    element.addEventListener('scroll', track)
    return () => {
      observer.disconnect()
      element.removeEventListener('scroll', track)
    }
  }, [])
  return ref
}

const contextHidden = 'follow-ups:context-hidden'

// Context is what we know of the person, as its markdown says it, to edit in
// place or fold away for every follow-up.
function Context({ record }: { record: CrmRecord }) {
  const context = text(record, 'context')
  const [hidden, setHidden] = useState(() => readItem(contextHidden) === 'true')
  const [editing, setEditing] = useState(false)
  const action = 'text-[12px] text-ink-3 outline-none hover:text-ink focus-visible:text-ink'
  return (
    <section aria-label={`${recordName(record)} context`} className="shrink-0 border-b border-border bg-surface px-6 py-3">
      <div className="flex items-center gap-3">
        <h3 className="text-[11.5px] font-semibold uppercase tracking-[0.04em] text-ink-3">Context</h3>
        {!editing && <button type="button" className={cn(action, 'ml-auto')} onClick={() => setEditing(true)}>{context ? 'Edit' : 'Add'}</button>}
        {context && !editing && <button type="button" aria-expanded={!hidden} className={action} onClick={() => {
          writeItem(contextHidden, String(!hidden))
          setHidden(!hidden)
        }}>{hidden ? 'Show' : 'Hide'}</button>}
      </div>
      {editing
        ? <ContextEditor record={record} autoFocus onDone={() => setEditing(false)} className="mt-1.5 min-h-16" />
        : context && !hidden && <div className="scrollbar-quiet mt-1.5 max-h-40 overflow-y-auto"><MarkdownView text={context} /></div>}
    </section>
  )
}

const drafted = { drafting: 'Drafting a reply…', completed: 'Reply drafted', failed: 'Couldn’t draft a reply', skipped: 'No reply drafted' }

function Draft({ record, channel, sender, error, drafting, participants }: { record: CrmRecord; channel: string; sender?: DraftSender; error?: string; drafting?: Interaction['drafting']; participants: Party[] }) {
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
  const named = (addresses: string[]) => addresses.map((address) => participants.find((p) => p.address === address)?.name || address).join(', ')
  // The AI's state shows while it works or could not help, and beside the
  // draft it wrote; addresses and actions show once there is a reply to send.
  const status = drafting && (drafting.state !== 'completed' || current) ? drafting : undefined
  const idle = !(draft || error || status) && 'hidden group-focus-within:flex'
  return (
    <div className="group rounded-[var(--radius-card)] bg-list-hover transition-colors duration-150 focus-within:bg-list-active">
      {email && (error || sender) && <p className={cn('flex min-w-0 gap-3 border-b border-border px-3.5 py-2 text-[12.5px] text-ink-3', idle)}>
        {error ? <span className="text-ink-2">{error}</span> : sender && <>
          <span className="min-w-0 flex-1 truncate" title={[...sender.to, ...sender.cc].join(', ')}>
            {sender.cc.length > 0 || sender.to.length > 1 ? 'Reply all' : 'Reply'} · to {named(sender.to)}{sender.cc.length > 0 && `, cc ${named(sender.cc)}`}
          </span>
          <span className="min-w-0 max-w-[40%] shrink-0 truncate" title={sender.from}>from {sender.from}</span>
        </>}
      </p>}
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
        className="field-sizing-content block max-h-[40dvh] min-h-10 w-full resize-none bg-transparent px-3.5 py-2.5 text-[13.5px] leading-[1.55] text-ink-2 outline-none placeholder:text-ink-3 focus:text-ink"
      />
      <div className={cn('flex items-center gap-2 py-2 pl-3.5 pr-2', idle)}>
        <p role="status" aria-live="polite" className="flex min-w-0 flex-1 items-start gap-1.5 text-[12px] leading-[18px] text-ink-3">
          {status?.state === 'drafting' && <LoaderCircle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 animate-spin motion-reduce:animate-none" />}
          {status ? <span>
            <span className={cn(status.state === 'failed' ? 'text-danger' : 'text-ink-2')}>{drafted[status.state]}</span>
            {status.reason && <> · {status.reason}</>}
          </span> : write.isPending ? 'Saving…' : current && edited === null && 'Draft saved'}
        </p>
        <Release record={record} draft={draft} channel={channel} sender={sender} beforeSend={commit} disabled={!!error} />
      </div>
    </div>
  )
}
