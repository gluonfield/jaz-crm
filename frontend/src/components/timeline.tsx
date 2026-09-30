import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { formatDateTime, timeAgo } from '@/lib/format'
import { useAction } from '@/lib/queries'
import type { Interaction, Kind } from '@/lib/types'
import { Button, Tab } from './controls'
import { KindIcon } from './icons'

export function participantsLine(interaction: Interaction) {
  return interaction.participants
    .filter((p) => p.role !== 'declined')
    .map((p) => p.name || p.address)
    .join(', ')
}

// InteractionRow is one entry of a timeline or a search result.
export function InteractionRow({ interaction }: { interaction: Interaction }) {
  return (
    <li>
      <Link
        to="/i/$interactionId"
        params={{ interactionId: interaction.id }}
        className="group flex gap-3 rounded-[var(--radius-card)] px-3 py-2.5 outline-none hover:bg-list-hover focus-visible:bg-list-hover"
      >
        <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full border border-border bg-raised text-ink-2">
          <KindIcon kind={interaction.kind} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex items-baseline gap-2">
            <span className="truncate text-[13px] font-medium text-ink">{interaction.title || 'No subject'}</span>
            <time className="ml-auto shrink-0 text-[12px] tabular-nums text-ink-3" dateTime={interaction.started_at} title={formatDateTime(interaction.started_at)}>
              {timeAgo(interaction.started_at)}
            </time>
          </span>
          <span className="block truncate text-[12px] text-ink-3">{participantsLine(interaction)}</span>
          {interaction.preview && <span className="mt-1 line-clamp-2 text-[12.5px] leading-[1.45] text-ink-2">{interaction.preview}</span>}
        </span>
      </Link>
    </li>
  )
}

const kinds: { kind: Kind; label: string }[] = [
  { kind: 'note', label: 'Note' },
  { kind: 'call', label: 'Call' },
  { kind: 'meeting', label: 'Meeting' },
]

// Composer logs a note, call or meeting against a record.
export function Composer({ recordId, people }: { recordId: string; people: string[] }) {
  const [kind, setKind] = useState<Kind>('note')
  const [notes, setNotes] = useState('')
  const log = useAction<object>('log_interaction')
  const submit = () => {
    if (notes.trim()) {
      log.mutate({ kind, notes: notes.trim(), records: [recordId], people: kind === 'note' ? [] : people }, { onSuccess: () => setNotes('') })
    }
  }
  return (
    <div className="rounded-[var(--radius-card)] border border-border bg-raised p-2">
      <textarea
        value={notes}
        onChange={(e) => setNotes(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            submit()
          }
        }}
        rows={2}
        placeholder={kind === 'note' ? 'Write a note...' : `What was said on the ${kind}?`}
        className="w-full resize-none bg-transparent px-1.5 py-1 text-[13px] leading-[1.5] text-ink outline-none placeholder:text-ink-3"
      />
      <div className="flex items-center gap-1">
        {kinds.map((k) => (
          <Tab key={k.kind} active={kind === k.kind} onClick={() => setKind(k.kind)}>
            <KindIcon kind={k.kind} />
            {k.label}
          </Tab>
        ))}
        <Button primary className="ml-auto" disabled={!notes.trim() || log.isPending} onClick={submit}>
          Log
        </Button>
      </div>
    </div>
  )
}
