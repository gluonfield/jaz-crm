import { Link } from '@tanstack/react-router'
import { EyeOff, Link2, Video, X } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { Chip, Header } from '@/components/controls'
import { MessageThread } from '@/components/email-thread'
import { KindIcon, RecordIcon } from '@/components/icons'
import { ExternalLink } from '@/components/external-link'
import { Message } from '@/components/message'
import { Picker } from '@/components/picker'
import { channelNames, joinURL, recordName } from '@/lib/crm'
import { formatDateTime, meetingTime } from '@/lib/format'
import { useDebounced } from '@/lib/hooks'
import { useAction, useRecordSearch, useTool } from '@/lib/queries'
import type { Interaction, Speech } from '@/lib/types'
import { cn } from '@/lib/utils'

const kindNames = { message: 'Message', meeting: 'Meeting', call: 'Call', note: 'Note' }

export function InteractionDetails({ interactionId, onClose, onNavigate }: { interactionId: string; onClose: () => void; onNavigate?: () => void }) {
  const query = useTool<Interaction>('get_interaction', { interaction_id: interactionId })
  const interaction = query.data
  const close = <Button variant="ghost" size="icon" aria-label="Close" onClick={onClose}><X /></Button>
  const link = useAction<object>('link_interaction')
  const unlink = useAction<object>('unlink_interaction')
  const skip = useAction<object>('skip_interaction')
  const [search, setSearch] = useState('')
  const found = useRecordSearch(useDebounced(search), 8)
  if (!interaction) {
    return (
      <>
        <Header>Conversation<span className="ml-auto">{close}</span></Header>
        <p role={query.isError ? 'alert' : 'status'} className="px-6 py-8 text-[13px] text-ink-3">{query.error?.message ?? 'Loading…'}</p>
      </>
    )
  }
  const ids = { interaction_id: interaction.id }
  const join = joinURL(interaction)
  return (
    <>
      <Header>
        <KindIcon kind={interaction.kind} className="size-4 text-ink-2" />
        {interaction.channel ? (channelNames[interaction.channel] ?? interaction.channel) : kindNames[interaction.kind]}
        <Button
          className="ml-auto"
          onClick={() => skip.mutate(ids, { onSuccess: onClose })}
          disabled={skip.isPending}
          title="Remove this conversation and its content from the CRM"
        >
          <EyeOff /> Remove
        </Button>
        {close}
      </Header>
      <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
        <article className="mx-auto max-w-[760px] px-4 pb-20 pt-8 sm:px-8">
          {interaction.title !== kindNames[interaction.kind] && <h1 className="text-[22px] font-semibold leading-tight tracking-[-0.015em] text-ink [overflow-wrap:anywhere]">{interaction.title || 'No subject'}</h1>}
          <p className="mt-1.5 text-[13px] text-ink-2">
            {interaction.author && <>{interaction.author} · </>}{interaction.kind === 'meeting' ? meetingTime(interaction.started_at, interaction.ended_at) : formatDateTime(interaction.started_at)}
          </p>
          {join && (
            <ExternalLink
              href={join}
              className="mt-4 inline-flex h-8 items-center gap-2 rounded-full bg-primary px-3.5 text-[13px] font-medium text-on-primary no-underline hover:bg-primary-strong hover:text-on-primary hover:no-underline"
            >
              <Video className="size-4" /> Join meeting
            </ExternalLink>
          )}
          <People interaction={interaction} onNavigate={onNavigate} />
          <div className="mt-3 flex flex-wrap items-center gap-1.5">
            {interaction.records.map((r) => (
              <Chip key={r.id} onRemove={() => unlink.mutate({ ...ids, record_id: r.id })}>
                <Link to="/r/$recordId" params={{ recordId: r.id }} onClick={onNavigate} className="flex items-center gap-1.5 hover:text-ink">
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
            <Content interaction={interaction} onNavigate={onNavigate} />
          </div>
        </article>
      </div>
    </>
  )
}

// People lists who took part, the organizer first; declined invitees are
// struck through.
function People({ interaction, onNavigate }: { interaction: Interaction; onNavigate?: () => void }) {
  const people = [...interaction.participants].sort((a, b) => Number(b.role === 'organizer') - Number(a.role === 'organizer'))
  return (
    <ul className="mt-5 flex flex-wrap gap-x-4 gap-y-2 text-[12.5px]">
      {people.map((p) => (
        <li key={p.address} title={`${p.address} · ${p.role}`} className={cn('inline-flex items-center gap-1.5 text-ink-2', p.role === 'declined' && 'text-ink-3 line-through')}>
          <RecordIcon object="people" name={p.name || p.address} photo={p.photo} size={20} />
          {p.person_id ? (
            <Link to="/r/$recordId" params={{ recordId: p.person_id }} onClick={onNavigate} className="hover:text-ink hover:underline">
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

function Content({ interaction, onNavigate }: { interaction: Interaction; onNavigate?: () => void }) {
  return (
    <div className="flex flex-col gap-6">
      {interaction.drafts?.map((draft) => (
        <Link key={draft.follow_up_id} to="/r/$recordId" params={{ recordId: draft.follow_up_id }} onClick={onNavigate} className="rounded-[var(--radius-control)] bg-panel px-3 py-2.5 hover:bg-list-hover">
          <span className="block text-[12px] text-ink-3">{draft.state === 'draft' ? 'Gmail draft · Review' : 'Draft removed from Gmail'}</span>
          <span className="text-[13px] text-ink">{draft.subject || 'No subject'}</span>
        </Link>
      ))}
      {interaction.invitation && (
        <section>
          <h2 className="mb-2 text-[12px] font-medium text-ink-3">Invitation</h2>
          <Message text={interaction.invitation} />
        </section>
      )}
      {interaction.text && <Message text={interaction.text} />}
      {interaction.messages && <MessageThread interaction={interaction} messages={interaction.messages} />}
      {interaction.transcript && <Transcript lines={interaction.transcript} />}
      {interaction.provenance && (
        <details className="text-[12px] text-ink-3">
          <summary className="w-fit cursor-pointer hover:text-ink-2">Source details</summary>
          <div className="mt-3"><Message text={interaction.provenance} /></div>
        </details>
      )}
    </div>
  )
}

// Transcript runs a meeting's lines together by speaker.
function Transcript({ lines }: { lines: Speech[] }) {
  const turns: { speaker: string; lines: string[] }[] = []
  for (const line of lines) {
    const speaker = line.speaker
    const last = turns.at(-1)
    if (last?.speaker === speaker) {
      last.lines.push(line.text)
    } else {
      turns.push({ speaker, lines: [line.text] })
    }
  }
  return (
    <section>
      <h2 className="mb-3 text-[12px] font-medium text-ink-3">Transcript</h2>
      <div className="flex max-w-[68ch] flex-col gap-4 text-[13.5px] leading-[1.6]">
        {turns.map((turn, i) => (
          <div key={i}>
            {turn.speaker && <div className="text-[12.5px] font-medium text-ink">{turn.speaker}</div>}
            <p className="text-ink-2">{turn.lines.join(' ')}</p>
          </div>
        ))}
      </div>
    </section>
  )
}
