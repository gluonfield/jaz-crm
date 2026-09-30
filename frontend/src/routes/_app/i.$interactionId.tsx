import { Link, createFileRoute, useRouter } from '@tanstack/react-router'
import { EyeOff, Link2 } from 'lucide-react'
import { useState } from 'react'
import { Button, Chip, Header } from '@/components/controls'
import { KindIcon, RecordIcon } from '@/components/icons'
import { Picker } from '@/components/picker'
import { recordName } from '@/lib/crm'
import { formatDateTime } from '@/lib/format'
import { useDebounced } from '@/lib/hooks'
import { useAction, useRecordSearch, useTool } from '@/lib/queries'
import type { Interaction, Part } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/i/$interactionId')({ component: InteractionPage })

const kindNames = { email: 'Email', meeting: 'Meeting', call: 'Call', note: 'Note' }

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
        <div className="mx-auto max-w-[780px] px-10 pb-16 pt-8">
          <h1 className="text-[20px] font-semibold tracking-[-0.01em] text-ink">{interaction.title || 'No subject'}</h1>
          <p className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[12.5px] text-ink-3">
            <span>{formatDateTime(interaction.started_at)}</span>
            {interaction.participants.map((p) => (
              <span
                key={p.address}
                title={`${p.address} · ${p.role}`}
                className={cn('inline-flex items-center gap-1.5', p.role === 'declined' && 'line-through')}
              >
                <RecordIcon object="people" name={p.name || p.address} photo={p.photo} size={16} />
                {p.person_id ? (
                  <Link to="/r/$recordId" params={{ recordId: p.person_id }} className="text-ink-2 hover:text-ink hover:underline">
                    {p.name || p.address}
                  </Link>
                ) : (
                  p.name || p.address
                )}
              </span>
            ))}
          </p>
          <div className="mt-5 flex flex-wrap items-center gap-1.5">
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
          <div className="mt-8 flex flex-col gap-3">
            <Parts parts={interaction.parts ?? []} />
            {interaction.source === 'calendar' && !interaction.parts?.some((p) => p.kind === 'transcript') && (
              <p className="text-[12.5px] text-ink-3">If this meeting is transcribed in Google Meet, the transcript appears here within a day.</p>
            )}
          </div>
        </div>
      </div>
    </>
  )
}

// Parts shows messages and notes as cards and a transcript as one dialogue.
function Parts({ parts }: { parts: Part[] }) {
  const transcript = parts.filter((p) => p.kind === 'transcript')
  return (
    <>
      {parts
        .filter((p) => p.kind !== 'transcript')
        .map((p, index) => (
          <article key={index} className="rounded-[var(--radius-card)] border border-border bg-raised px-4 py-3">
            <header className="mb-2 flex items-baseline gap-2 text-[12.5px]">
              <span className="font-medium text-ink">{p.author || (p.kind === 'description' ? 'Agenda' : 'Note')}</span>
              <time className="ml-auto text-ink-3">{formatDateTime(p.at)}</time>
            </header>
            {p.content ? (
              <p className="whitespace-pre-wrap break-words text-[13px] leading-[1.55] text-ink-2">{p.content}</p>
            ) : (
              <p className="text-[12.5px] text-ink-3">Not fetched yet</p>
            )}
          </article>
        ))}
      {transcript.length > 0 && (
        <article className="rounded-[var(--radius-card)] border border-border bg-raised px-4 py-3">
          <header className="mb-2 text-[12.5px] font-medium text-ink">Transcript</header>
          <div className="flex flex-col gap-1.5 text-[13px] leading-[1.55]">
            {transcript.map((line, index) => (
              <p key={index} className="whitespace-pre-wrap break-words text-ink-2">
                {line.author && <span className="font-medium text-ink">{line.author} </span>}
                {line.content}
              </p>
            ))}
          </div>
        </article>
      )}
    </>
  )
}
