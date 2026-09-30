import { Link, createFileRoute, useRouter } from '@tanstack/react-router'
import { EyeOff, Link2, Video } from 'lucide-react'
import { useState } from 'react'
import { Button, Chip, Header } from '@/components/controls'
import { EmailThread } from '@/components/email-thread'
import { KindIcon, RecordIcon } from '@/components/icons'
import { ExternalLink, Message } from '@/components/message'
import { Picker } from '@/components/picker'
import { recordName } from '@/lib/crm'
import { formatDateTime, hasEnded, meetingTime } from '@/lib/format'
import { useDebounced } from '@/lib/hooks'
import { useAction, useRecordSearch, useTool } from '@/lib/queries'
import type { Interaction, Part } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/i/$interactionId')({ component: InteractionPage })

const kindNames = { email: 'Email', meeting: 'Meeting', call: 'Call', note: 'Note' }

const joinLink = /https?:\/\/(teams\.microsoft\.com|[\w.-]*zoom\.us|meet\.google\.com)\/\S+/

function InteractionPage() {
  const { interactionId } = Route.useParams()
  const interaction = useTool<Interaction>('get_interaction', { interaction_id: interactionId }).data
  const router = useRouter()
  const link = useAction<object>('link_interaction')
  const unlink = useAction<object>('unlink_interaction')
  const skip = useAction<object>('skip_interaction')
  const [search, setSearch] = useState('')
  const found = useRecordSearch(useDebounced(search), 8)
  if (!interaction) {
    return <Header />
  }
  const ids = { interaction_id: interaction.id }
  const parts = interaction.parts ?? []
  const join = interaction.kind === 'meeting' && !hasEnded(interaction.ended_at ?? interaction.started_at) ? parts.map((p) => p.content.match(joinLink)?.[0]).find(Boolean) : undefined
  return (
    <>
      <Header>
        <KindIcon kind={interaction.kind} className="size-4 text-ink-2" />
        {kindNames[interaction.kind]}
        <Button
          className="ml-auto"
          onClick={() => skip.mutate(ids, { onSuccess: () => router.history.back() })}
          title="Remove this conversation and its content from the CRM"
        >
          <EyeOff /> Remove
        </Button>
      </Header>
      <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
        <article className="mx-auto max-w-[760px] px-4 pb-20 pt-8 sm:px-8">
          <h1 className="text-[22px] font-semibold leading-tight tracking-[-0.015em] text-ink">{interaction.title || 'No subject'}</h1>
          <p className="mt-1.5 text-[13px] text-ink-2">
            {interaction.kind === 'meeting' ? meetingTime(interaction.started_at, interaction.ended_at) : formatDateTime(interaction.started_at)}
          </p>
          {join && (
            <ExternalLink
              href={join}
              className="mt-4 inline-flex h-8 items-center gap-2 rounded-full bg-primary px-3.5 text-[13px] font-medium text-on-primary no-underline hover:bg-primary-strong"
            >
              <Video className="size-4" /> Join meeting
            </ExternalLink>
          )}
          <People interaction={interaction} />
          <div className="mt-3 flex flex-wrap items-center gap-1.5">
            {interaction.records.map((r) => (
              <Chip key={r.id} onRemove={() => unlink.mutate({ ...ids, record_id: r.id })}>
                <Link to="/r/$recordId" params={{ recordId: r.id }} className="flex items-center gap-1.5 hover:text-ink">
                  <RecordIcon object={r.object} name={r.name ?? ''} size={14} />
                  {r.name}
                </Link>
              </Chip>
            ))}
            <Picker
              trigger={
                <button className="inline-flex h-[22px] items-center gap-1 rounded-full px-2 text-[12px] text-ink-3 outline-none hover:bg-list-hover hover:text-ink">
                  <Link2 className="size-3" /> Link record
                </button>
              }
              placeholder="Find a record..."
              onSearch={setSearch}
              options={found.map((r) => ({ value: r.id, label: recordName(r), icon: <RecordIcon object={r.object} name={recordName(r)} size={16} /> }))}
              onSelect={(id) => link.mutate({ ...ids, record_id: id })}
            />
          </div>
          <div className="mt-8 border-t border-border pt-6">
            <Content interaction={interaction} parts={parts} />
          </div>
        </article>
      </div>
    </>
  )
}

// People lists who took part, the organizer first; declined invitees are
// struck through.
function People({ interaction }: { interaction: Interaction }) {
  const people = [...interaction.participants].sort((a, b) => Number(b.role === 'organizer') - Number(a.role === 'organizer'))
  return (
    <ul className="mt-5 flex flex-wrap gap-x-4 gap-y-2 text-[12.5px]">
      {people.map((p) => (
        <li key={p.address} title={`${p.address} · ${p.role}`} className={cn('inline-flex items-center gap-1.5 text-ink-2', p.role === 'declined' && 'text-ink-3 line-through')}>
          <RecordIcon object="people" name={p.name || p.address} photo={p.photo} size={20} />
          {p.person_id ? (
            <Link to="/r/$recordId" params={{ recordId: p.person_id }} className="hover:text-ink hover:underline">
              {p.name || p.address}
            </Link>
          ) : (
            (p.name || p.address)
          )}
          {p.role === 'organizer' && <span className="text-ink-3">organizer</span>}
        </li>
      ))}
    </ul>
  )
}

// Content shows what was said: an email thread as messages, a meeting's
// agenda and transcript, and notes.
function Content({ interaction, parts }: { interaction: Interaction; parts: Part[] }) {
  const transcript = parts.filter((p) => p.kind === 'transcript')
  const agenda = parts.find((p) => p.kind === 'description')
  const messages = parts.filter((p) => p.kind !== 'transcript' && p.kind !== 'description')
  const meet = interaction.source === 'calendar' && parts.some((p) => p.content.includes('meet.google.com'))
  if (parts.length === 0) {
    return <p className="text-[13px] text-ink-3">Nothing was written down.</p>
  }
  return (
    <div className="flex flex-col gap-8">
      {agenda?.content && (
        <section>
          <h2 className="mb-2 text-[12px] font-medium text-ink-3">Invitation</h2>
          <Message text={agenda.content} />
        </section>
      )}
      {messages.length > 0 && (interaction.kind === 'email' ? <EmailThread interaction={interaction} messages={messages} /> : <Thread interaction={interaction} messages={messages} />)}
      {transcript.length > 0 ? (
        <Transcript lines={transcript} />
      ) : (
        meet && <p className="text-[12.5px] text-ink-3">If this meeting is transcribed in Google Meet, the transcript appears here within a day.</p>
      )}
    </div>
  )
}

// Thread shows an email conversation; with several messages, all but the
// latest start folded to a line.
function Thread({ interaction, messages }: { interaction: Interaction; messages: Part[] }) {
  const [open, setOpen] = useState(() => new Set([messages.length - 1]))
  const photo = (author?: string) => interaction.participants.find((p) => (p.name || p.address) === author)?.photo
  return (
    <ol className="flex flex-col">
      {messages.map((m, i) => {
        const folded = !open.has(i)
        const author = m.author || (m.kind === 'note' ? 'Note' : 'Unknown sender')
        return (
          <li key={i} className={cn('border-border', i > 0 && 'border-t', folded ? 'py-2.5' : 'py-5 first:pt-0')}>
            <button
              type="button"
              aria-expanded={!folded}
              onClick={() => setOpen((current) => new Set(current).add(i))}
              disabled={!folded}
              className="flex w-full min-w-0 items-center gap-2.5 rounded-[var(--radius-control)] text-left outline-none focus-visible:ring-2 focus-visible:ring-ring enabled:hover:bg-list-hover"
            >
              <RecordIcon object="people" name={author} photo={photo(m.author)} size={folded ? 22 : 28} />
              <span className="shrink-0 text-[13px] font-medium text-ink">{author}</span>
              {folded && <span className="min-w-0 flex-1 truncate text-[12.5px] text-ink-3">{m.content.replace(/\s+/g, ' ')}</span>}
              <time className="ml-auto shrink-0 pl-3 text-[12px] tabular-nums text-ink-3">{formatDateTime(m.at)}</time>
            </button>
            {!folded && (
              <div className="mt-3 pl-[38px]">{m.content ? <Message text={m.content} /> : <p className="text-[13px] text-ink-3">The text arrives with the next sync.</p>}</div>
            )}
          </li>
        )
      })}
    </ol>
  )
}

// Transcript runs a meeting's lines together by speaker.
function Transcript({ lines }: { lines: Part[] }) {
  const turns: { speaker: string; lines: string[] }[] = []
  for (const line of lines) {
    const speaker = line.author || 'Unknown speaker'
    const last = turns.at(-1)
    if (last?.speaker === speaker) {
      last.lines.push(line.content)
    } else {
      turns.push({ speaker, lines: [line.content] })
    }
  }
  return (
    <section>
      <h2 className="mb-3 text-[12px] font-medium text-ink-3">Transcript</h2>
      <div className="flex max-w-[68ch] flex-col gap-4 text-[13.5px] leading-[1.6]">
        {turns.map((turn, i) => (
          <div key={i}>
            <div className="text-[12.5px] font-medium text-ink">{turn.speaker}</div>
            <p className="text-ink-2">{turn.lines.join(' ')}</p>
          </div>
        ))}
      </div>
    </section>
  )
}
