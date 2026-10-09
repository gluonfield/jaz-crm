import { ArrowLeft, Check, Circle, CircleCheck, CircleDot, CircleX, Clock, LoaderCircle, Mail, NotebookText } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { type ReactNode, useEffect, useRef, useState } from 'react'
import { Button } from '@jaz/ui/button'
import { recordName, textOf, valueText, valuesOf } from '@/lib/crm'
import { dueLabel, formatActionDate } from '@/lib/dates'
import { standingOf, statusOf } from '@/lib/follow-ups'
import { formatDate, meetingTime } from '@/lib/format'
import { useRecordPages, useTool, useWorkspace, useWrite } from '@/lib/queries'
import { readItem, writeItem } from '@/lib/storage'
import type { CrmRecord, DraftSender, Interaction, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { RecordChip } from './controls'
import { DateField } from './date-field'
import { Divider, Happening, MessageThread } from './email-thread'
import { MarkdownView } from './editor'
import { ExternalLink } from './external-link'
import { ChannelIcon, KindIcon, RecordIcon } from './icons'
import { ContextEditor } from './person-context'
import { Draft, type DraftHandle } from './draft'

const subjects = { person: 'people', company: 'companies', deal: 'deals' }
const channels: Record<string, string> = { email: 'Email', linkedin: 'LinkedIn' }

// refsOf lists whom a follow-up concerns, its person first.
export const refsOf = (record: CrmRecord) => Object.entries(subjects).flatMap(([object, plural]) => (valuesOf(record, object) as Ref[]).map((ref) => ({ plural, ref })))

// editDraft starts editing the open follow-up's draft.
export const editDraft = () => document.querySelector<HTMLElement>('[data-conversation] textarea[aria-label="Draft"]')?.focus()

export function Done({ record, labelled = false, className }: { record: CrmRecord; labelled?: boolean; className?: string }) {
  const write = useWrite(record)
  const status = statusOf(record)
  if (status !== 'Open') {
    return labelled ? <span className="text-[12px] text-ink-3">{status}</span> : null
  }
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

// useSource loads what a follow-up came from: its message conversation, or
// without one, the meeting or call behind it.
function useSource(record: CrmRecord) {
  const scoped = !!record.conversation_id
  const messages = useTool<{ interactions: Interaction[] }>('list_interactions', { record_id: record.id, kinds: ['message'], limit: 1 }, { enabled: !scoped })
  const meetings = useTool<{ interactions: Interaction[] }>('list_interactions', { record_id: record.id, kinds: ['meeting', 'call'], limit: 1 }, { enabled: !scoped })
  const found = messages.data?.interactions[0] ?? meetings.data?.interactions[0]
  const id = record.conversation_id ?? found?.id
  const thread = useTool<Interaction>('get_interaction', { interaction_id: id ?? '' }, {
    enabled: !!id,
    refetchInterval: (query) => (query.state.data?.drafting?.state === 'drafting' ? 3000 : false),
  })
  const source = thread.data ?? found
  return {
    source,
    // A conversation_id is the follow-up's one message conversation.
    written: scoped || source?.kind === 'message',
    said: thread.data?.messages ?? (source?.last_message ? [source.last_message] : []),
    // A query without data reads as pending again on every refetch, even
    // after it failed, so loading means not yet answered.
    loading: id ? !thread.isFetched : !messages.isFetched || !meetings.isFetched,
    error: messages.error ?? meetings.error ?? thread.error,
  }
}

const sourceTitle = (source: Interaction) => source.title || (source.kind === 'message' ? 'Conversation' : source.kind === 'call' ? 'Call' : 'Meeting')

function SourceIcon({ source }: { source: Interaction }) {
  if (source.kind !== 'message') {
    return <KindIcon kind={source.kind} />
  }
  return source.channel && source.channel !== 'email' ? <ChannelIcon channel={source.channel} className="size-3" /> : <Mail className="size-3.5 shrink-0" />
}

// Conversation is the open follow-up: what to do and by when, what it came
// from, a reply box, and beside them who it is with and the steps taken with
// them so far.
export function Conversation({ record: initialRecord, onClose }: { record: CrmRecord; onClose: () => void }) {
  const zone = useWorkspace()?.timezone ?? 'UTC'
  const { source, written, said, loading, error: sourceError } = useSource(initialRecord)
  const actions = useRecordPages({ object: 'follow_ups', conversation_id: initialRecord.conversation_id, limit: 100 }, { enabled: !!initialRecord.conversation_id, all: true })
  const [selectedAction, setSelectedAction] = useState(initialRecord)
  const record = initialRecord.conversation_id
    ? actions.records?.find((action) => action.id === selectedAction.id) ?? selectedAction
    : initialRecord
  if (record !== selectedAction) {
    setSelectedAction(record)
  }
  const [switching, setSwitching] = useState(false)
  const draft = useRef<DraftHandle>(null)
  const selectAction = async (action: CrmRecord) => {
    setSwitching(true)
    try {
      if (await draft.current?.save()) {
        setSelectedAction(action)
      }
    } catch {
      // The current editor retains its text and displays the save error.
    } finally {
      setSwitching(false)
    }
  }
  const [subject] = refsOf(record)
  const person = valuesOf(record, 'person')[0] as Ref | undefined
  const company = valuesOf(record, 'company')[0] as Ref | undefined
  const context = useTool<CrmRecord>('get_record', { record_id: person?.id ?? '' }, { enabled: !!person })
  const drafting = source?.drafting
  const draftingState = drafting?.state
  const draftingStartedAt = drafting?.started_at
  const client = useQueryClient()
  useEffect(() => {
    if (draftingState && draftingState !== 'drafting') {
      void client.invalidateQueries({ queryKey: ['search_records'] })
    }
  }, [draftingState, draftingStartedAt, client])
  const channel = textOf(record, 'channel') || channels[source?.channel ?? ''] || (written ? '' : 'Email')
  const sender = useTool<DraftSender>('get_draft_sender', { record_id: record.id }, { enabled: channel === 'Email' })
  const error = sourceError ?? actions.error
  // Whether details show on a narrow screen is one choice for every follow-up.
  const [showDetails, setShowDetails] = useState(() => readItem(detailsHidden) !== 'true')
  const created = <>Follow-up created · {recordName(record)}</>
  const body = useStickToBottom()
  const steps = actions.records ?? [record]
  const due = textOf(record, 'action_date')
  return (
    <section data-conversation aria-label={recordName(record)} className="flex min-w-0 flex-1 flex-col">
      <header className="flex shrink-0 items-start gap-3 border-b border-border py-3 pl-6 pr-4">
        <Button variant="ghost" size="icon-sm" aria-label="Back" title="Back" className="-ml-2 mt-0.5 @4xl:hidden" onClick={onClose}>
          <ArrowLeft />
        </Button>
        <div className="min-w-0 flex-1 py-0.5">
          <h2 className="text-[15px] font-semibold leading-[1.35] text-ink [text-wrap:balance]">{recordName(record)}</h2>
          <p className="mt-1 flex min-w-0 items-center gap-1.5 text-[12px] text-ink-3">
            {subject && <span className="flex min-w-0 shrink items-center gap-1.5 @min-[78rem]:hidden">
              <Link to="/r/$recordId" params={{ recordId: subject.ref.id }} className="truncate text-ink-2 outline-none hover:underline focus-visible:underline">{subject.ref.name || 'Unnamed'}</Link>
              {source && <span aria-hidden="true">·</span>}
            </span>}
            {source && <>
              <SourceIcon source={source} />
              <span className="min-w-0 truncate">{sourceTitle(source)}</span>
              {said.length > 1 && <span className="shrink-0 tabular-nums">· {said.length} messages</span>}
            </>}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <DateField record={record} prefix="Due" />
          <Done record={record} labelled className="bg-list-hover hover:bg-list-active" />
          <Button variant="ghost" size="icon-sm" aria-label="Details" title={showDetails ? 'Hide details' : 'Show details'} aria-pressed={showDetails} className={cn('shrink-0 @min-[78rem]:hidden', showDetails && 'bg-list-active text-ink hover:bg-list-active')} onClick={() => {
            writeItem(detailsHidden, String(showDetails))
            setShowDetails(!showDetails)
          }}>
            <NotebookText />
          </Button>
        </div>
      </header>
      <div className="flex min-h-0 flex-1 flex-col @min-[78rem]:flex-row">
        <aside aria-label="Details" className={cn('scrollbar-quiet flex max-h-[40%] shrink-0 flex-col gap-5 overflow-y-auto border-b border-border bg-surface px-6 py-4 @min-[78rem]:order-last @min-[78rem]:max-h-none @min-[78rem]:w-[288px] @min-[78rem]:gap-7 @min-[78rem]:border-b-0 @min-[78rem]:border-l @min-[78rem]:bg-transparent @min-[78rem]:px-5 @min-[78rem]:py-5', !showDetails && 'hidden @min-[78rem]:flex')}>
          {subject && <Profile subject={subject} details={context.data} company={person ? company : undefined} />}
          {context.data && (
            <section aria-label="About">
              <Heading>About</Heading>
              <Context record={context.data} />
            </section>
          )}
          {steps.length > 1 && <Steps steps={steps} selected={record} pending={switching} zone={zone} onSelect={(action) => void selectAction(action)} />}
        </aside>
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
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
                  {source && written && said.length > 0
                    ? <MessageThread key={source.id} interaction={source} messages={said} events={[{ at: record.created_at, label: created }]} initialVisible={4} />
                    : <>
                      {source && !written && <Meeting source={source} />}
                      {source && written && !source.drafts?.length && <p className="text-[12px] text-ink-3">Message text is not available yet.</p>}
                      <Divider>{formatDate(record.created_at)}</Divider>
                      <Happening>{created}</Happening>
                    </>}
                </>
              )}
            </div>
          </div>
          <footer className="shrink-0 px-6 pb-5 pt-2 @4xl:px-8">
            {!loading && standingOf(record, zone) === 'waiting' && <p className="mb-2 flex min-w-0 items-center gap-2 px-1 text-[12.5px] text-ink-3">
              <Clock aria-hidden="true" className="size-3.5 shrink-0" />
              <span className="truncate">Waiting for {firstName(person?.name) || 'a reply'}{due && <> · chase {formatActionDate(due, zone)}</>}</span>
            </p>}
            <Draft key={record.id} ref={draft} record={record} channel={channel} sender={sender.data} error={sender.error?.message} drafting={drafting} />
          </footer>
        </div>
      </div>
    </section>
  )
}

const firstName = (name?: string) => name?.split(/[\s@]/)[0]
const byCreation = (a: CrmRecord, b: CrmRecord) => Date.parse(a.created_at) - Date.parse(b.created_at)

function Heading({ children }: { children: ReactNode }) {
  return <h3 className="mb-2.5 text-[11px] font-semibold uppercase tracking-[0.04em] text-ink-3">{children}</h3>
}

// Profile is whom the follow-up is with: who they are and where to reach
// them. A narrow screen names them in the header instead.
function Profile({ subject, details, company }: { subject: { plural: string; ref: Ref }; details?: CrmRecord; company?: Ref }) {
  const role = details && textOf(details, 'job_title')
  const email = details && valuesOf(details, 'email_addresses').map(valueText)[0]
  const linkedin = details && textOf(details, 'linkedin_url')
  return (
    <section aria-label="Profile" className="hidden flex-col items-start @min-[78rem]:flex">
      <RecordIcon object={subject.plural} name={subject.ref.name ?? ''} photo={subject.ref.photo} size={40} />
      <Link to="/r/$recordId" params={{ recordId: subject.ref.id }} className="mt-3 max-w-full truncate text-[14px] font-semibold text-ink outline-none hover:underline focus-visible:underline">{subject.ref.name || 'Unnamed'}</Link>
      {role && <p className="mt-0.5 max-w-full text-[12px] text-ink-3">{role}</p>}
      {company && <div className="mt-2 max-w-full"><RecordChip object="companies" value={company} /></div>}
      {(email || linkedin) && <div className="mt-3 flex max-w-full flex-wrap gap-1.5 text-[12px]">
        {email && <a href={`mailto:${email}`} title={email} className="flex h-6 max-w-full items-center gap-1.5 rounded-[var(--radius-control)] bg-list-hover px-2 text-ink-2 outline-none hover:bg-list-active hover:text-ink focus-visible:ring-2 focus-visible:ring-ring"><Mail className="size-3.5 shrink-0" /><span className="truncate">{email}</span></a>}
        {linkedin && <ExternalLink href={linkedin} className="flex h-6 items-center gap-1.5 rounded-[var(--radius-control)] bg-list-hover px-2 text-ink-2 hover:bg-list-active hover:text-ink"><ChannelIcon channel="linkedin" className="size-3" />LinkedIn</ExternalLink>}
      </div>}
    </section>
  )
}

// Steps are the follow-ups of one conversation in the order they arose, each
// open to review; dismissed ones wait behind a count.
function Steps({ steps, selected, pending, zone, onSelect }: { steps: CrmRecord[]; selected: CrmRecord; pending: boolean; zone: string; onSelect: (step: CrmRecord) => void }) {
  const [showDismissed, setShowDismissed] = useState(false)
  const dismissed = (step: CrmRecord) => statusOf(step) === 'Dismissed' && step.id !== selected.id
  const hidden = steps.filter(dismissed).length
  return (
    <section aria-label="Steps">
      <Heading>Steps</Heading>
      <ol className="flex flex-col">
        {steps.filter((step) => showDismissed || !dismissed(step)).sort(byCreation).map((step) => {
          const status = statusOf(step)
          const current = step.id === selected.id
          const due = textOf(step, 'action_date')
          const Icon = status === 'Done' ? CircleCheck : status === 'Dismissed' ? CircleX : current ? CircleDot : Circle
          return (
            <li key={step.id} className="relative flex gap-2.5 pb-3.5 before:absolute before:bottom-1 before:left-[7px] before:top-[20px] before:w-px before:bg-border last:pb-0 last:before:hidden">
              <Icon aria-hidden="true" className={cn('mt-px size-[15px] shrink-0', status === 'Done' ? 'text-ok' : current ? 'text-primary' : 'text-ink-3')} />
              <button type="button" disabled={current || pending} aria-current={current || undefined} onClick={() => onSelect(step)} className="group min-w-0 text-left text-[12.5px] leading-[1.4] outline-none focus-visible:underline disabled:cursor-default">
                <span className={cn('block group-enabled:group-hover:text-ink', current ? 'font-medium text-ink' : 'text-ink-2')}>{recordName(step)}</span>
                <span className="mt-0.5 block text-[11.5px] text-ink-3">{status !== 'Open' ? status : due ? dueLabel(due, zone).label : 'Open'}</span>
              </button>
            </li>
          )
        })}
      </ol>
      {hidden > 0 && <button type="button" onClick={() => setShowDismissed(!showDismissed)} className="mt-3 text-[12px] text-ink-3 outline-none hover:text-ink focus-visible:text-ink">{showDismissed ? 'Hide dismissed' : `${hidden} dismissed`}</button>}
    </section>
  )
}

// Meeting is the meeting or call a follow-up came from, as its notes begin,
// with the rest a click away.
function Meeting({ source }: { source: Interaction }) {
  const notes = source.text || source.preview
  return (
    <article className="rounded-[var(--radius-card)] bg-list-hover px-4 py-3.5">
      <header className="flex min-w-0 items-center gap-2 text-[13px]">
        <span className="flex text-ink-2"><SourceIcon source={source} /></span>
        <span className="min-w-0 truncate font-medium text-ink">{sourceTitle(source)}</span>
        <span className="shrink-0 text-[12px] text-ink-3">{meetingTime(source.started_at, source.ended_at)}</span>
        <Link to="/i/$interactionId" params={{ interactionId: source.id }} className="ml-auto shrink-0 text-[12px] text-ink-3 outline-none hover:text-ink focus-visible:underline">Open</Link>
      </header>
      {notes && <div className="mt-2.5 max-h-60 overflow-hidden pl-[22px] [mask-image:linear-gradient(to_bottom,black_65%,transparent)]"><MarkdownView text={notes} /></div>}
    </article>
  )
}

// useStickToBottom keeps a conversation at its latest message, as chat apps
// do, while its content or the reply box below changes size, until someone
// scrolls up to read or opens something in it.
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
    const release = () => {
      pinned = false
    }
    const observer = new ResizeObserver(follow)
    observer.observe(element)
    observer.observe(element.firstElementChild!)
    element.addEventListener('scroll', track)
    element.addEventListener('pointerdown', release)
    element.addEventListener('keydown', release)
    return () => {
      observer.disconnect()
      element.removeEventListener('scroll', track)
      element.removeEventListener('pointerdown', release)
      element.removeEventListener('keydown', release)
    }
  }, [])
  return ref
}

// The key keeps its earlier name so people's stored choice carries over.
const detailsHidden = 'follow-ups:context-hidden'

// Context is what we know of the person, as its markdown says it, to edit in
// place; without any it is a field to write it in.
function Context({ record }: { record: CrmRecord }) {
  const context = textOf(record, 'context')
  const [editing, setEditing] = useState(false)
  return (
    <section aria-label={`${recordName(record)} context`} className="flex items-start gap-3">
      {editing || !context
        ? <ContextEditor record={record} autoFocus={editing} onDone={() => setEditing(false)} className="min-h-12 flex-1 resize-none" />
        : <>
          <div className="min-w-0 flex-1"><MarkdownView text={context} /></div>
          <button type="button" className="shrink-0 text-[12px] text-ink-3 outline-none hover:text-ink focus-visible:text-ink" onClick={() => setEditing(true)}>Edit</button>
        </>}
    </section>
  )
}
