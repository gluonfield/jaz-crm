import { ArrowLeft, Check, LoaderCircle, NotebookText } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { Button } from '@jaz/ui/button'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { isOverdue } from '@/lib/dates'
import { useTool, useWorkspace, useWrite } from '@/lib/queries'
import { readItem, writeItem } from '@/lib/storage'
import type { CrmRecord, DraftSender, Interaction, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { RecordChip } from './controls'
import { DateField, DateLabel } from './date-field'
import { Divider, MessageThread } from './email-thread'
import { MarkdownView } from './editor'
import { ChannelTag, RecordIcon } from './icons'
import { ContextEditor } from './person-context'
import { Draft } from './draft'

export const text = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText).join(', ')

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
  const channel = text(record, 'channel') || channels[conversation?.channel ?? ''] || (!conversation ? 'Email' : '')
  const sender = useTool<DraftSender>('get_draft_sender', { record_id: record.id }, { enabled: channel === 'Email' })
  // A query without data reads as pending again on every refetch, even after
  // it failed, so loading means not yet answered.
  const loading = !conversations.isFetched || (!!conversation && !thread.isFetched)
  const messages = thread.data?.messages ?? (conversation?.last_message ? [conversation.last_message] : [])
  const error = conversations.error ?? thread.error
  const due = text(record, 'action_date')
  const zone = useWorkspace()?.timezone ?? 'UTC'
  // Whether context shows is one choice for every follow-up.
  const [showContext, setShowContext] = useState(() => readItem(contextHidden) !== 'true')
  const body = useStickToBottom()
  return (
    <section data-conversation aria-label={recordName(record)} className="flex min-w-0 flex-1 flex-col">
      <header className="flex shrink-0 items-center gap-3 border-b border-border py-3.5 pl-6 pr-4">
        <Button variant="ghost" size="icon-sm" aria-label="Back" title="Back" className="-ml-2 @4xl:hidden" onClick={onClose}>
          <ArrowLeft />
        </Button>
        {subject && <RecordIcon object={subject.plural} name={subject.ref.name ?? ''} photo={subject.ref.photo} size={32} />}
        <div className="flex min-w-0 flex-col gap-[3px]">
          <div className="flex min-w-0 items-center gap-2">
            {subject
              ? <Link to="/r/$recordId" params={{ recordId: subject.ref.id }} className="truncate text-[14px] font-semibold text-ink outline-none hover:underline focus-visible:underline">{subject.ref.name || 'Unnamed'}</Link>
              : <h2 className="truncate text-[14px] font-semibold text-ink">{recordName(record)}</h2>}
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
          {person && <Button variant="ghost" aria-pressed={showContext} className={cn(showContext && 'bg-list-active text-ink hover:bg-list-active')} onClick={() => {
            writeItem(contextHidden, String(showContext))
            setShowContext(!showContext)
          }}>
            <NotebookText />
            Context
          </Button>}
          <DateField record={record} prefix="Action" />
          <Done record={record} labelled className="bg-list-hover hover:bg-list-active" />
        </div>
      </header>
      {showContext && context.data && <Context record={context.data} />}
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
        <Draft record={record} channel={channel} sender={sender.data} error={sender.error?.message} drafting={drafting} />
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
// place; without any it is a field to write it in.
function Context({ record }: { record: CrmRecord }) {
  const context = text(record, 'context')
  const [editing, setEditing] = useState(false)
  return (
    <section aria-label={`${recordName(record)} context`} className="flex shrink-0 items-start gap-3 border-b border-border bg-surface px-6 py-3">
      {editing || !context
        ? <ContextEditor record={record} autoFocus={editing} onDone={() => setEditing(false)} className="min-h-12 flex-1 resize-none" />
        : <>
          <div className="scrollbar-quiet max-h-40 min-w-0 flex-1 overflow-y-auto"><MarkdownView text={context} /></div>
          <button type="button" className="shrink-0 text-[12px] text-ink-3 outline-none hover:text-ink focus-visible:text-ink" onClick={() => setEditing(true)}>Edit</button>
        </>}
    </section>
  )
}
