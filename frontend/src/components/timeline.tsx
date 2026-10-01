import { useState } from 'react'
import { formatDateTime, recentOrDate } from '@/lib/format'
import { useAction, useTimeline, useUpcoming } from '@/lib/queries'
import type { CrmObject, Interaction, Kind } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Button } from '@jaz/ui/button'
import { MessageState } from './email-thread'
import { KindIcon } from './icons'
import { InteractionDialog } from './interaction-dialog'

const filters: { label: string; kinds?: Kind[] }[] = [
  { label: 'All' },
  { label: 'Messages', kinds: ['message'] },
  { label: 'Meetings', kinds: ['meeting'] },
  { label: 'Notes and calls', kinds: ['note', 'call'] },
]

// synced objects gather conversations on their own.
const synced = ['people', 'companies']

const month = new Intl.DateTimeFormat('en', { month: 'long' })
const monthYear = new Intl.DateTimeFormat('en', { month: 'long', year: 'numeric' })
const weekday = new Intl.DateTimeFormat('en', { weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })

// monthOf heads a month of a timeline, naming the year only when it is not
// this one.
function monthOf(iso: string) {
  const date = new Date(iso.length === 10 ? `${iso}T00:00:00` : iso)
  return (date.getFullYear() === new Date().getFullYear() ? month : monthYear).format(date)
}

// byMonth splits a newest-first list into its months.
function byMonth(items: Interaction[]) {
  const months: [string, Interaction[]][] = []
  for (const item of items) {
    const name = monthOf(item.started_at)
    const last = months.at(-1)
    if (last?.[0] === name) {
      last[1].push(item)
    } else {
      months.push([name, [item]])
    }
  }
  return months
}

// Timeline is a record's conversations: what is scheduled, a composer for
// notes and calls, then everything that happened, newest first, by month.
export function Timeline({ recordId, object, people }: { recordId: string; object: CrmObject; people: string[] }) {
  const [filter, setFilter] = useState(filters[0])
  const timeline = useTimeline(recordId, filter.kinds)
  const upcoming = useUpcoming(recordId)
  const items = timeline.data?.pages.flatMap((p) => p.interactions) ?? []
  return (
    <div className="flex flex-col gap-6">
      <Composer recordId={recordId} people={people} />
      {upcoming.length > 0 && (
        <section>
          <h3 className="mb-1 text-[12px] font-medium text-ink-3">Upcoming</h3>
          <ol>
            {upcoming.map((i) => (
              <InteractionRow key={i.id} interaction={i} when={weekday.format(new Date(i.started_at))} />
            ))}
          </ol>
        </section>
      )}
      <nav aria-label="Show" className="-mb-3 flex flex-wrap gap-x-4 gap-y-1 text-[12.5px]">
        {filters.map((f) => (
          <button
            key={f.label}
            type="button"
            aria-pressed={f === filter}
            onClick={() => setFilter(f)}
            className={cn('rounded-[4px] outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring', f === filter ? 'font-medium text-ink' : 'text-ink-3 hover:text-ink-2')}
          >
            {f.label}
          </button>
        ))}
      </nav>
      {timeline.data && items.length === 0 && (
        <p className="text-[13px] leading-[1.5] text-ink-3">
          {filter.kinds
            ? `No ${filter.label.toLowerCase()} yet.`
            : synced.includes(object.slug)
              ? 'Emails and meetings appear here as they sync, with the notes and calls you log.'
              : `Link an email or meeting to this ${object.name.toLowerCase().replace(/s$/, '')} from its page, or log a note or call above.`}
        </p>
      )}
      {byMonth(items).map(([name, list]) => (
        <section key={name}>
          <h3 className="mb-1 text-[12px] font-medium text-ink-3">{name}</h3>
          <ol>
            {list.map((i) => (
              <InteractionRow key={i.id} interaction={i} />
            ))}
          </ol>
        </section>
      ))}
      {timeline.hasNextPage && (
        <Button className="self-start" disabled={timeline.isFetchingNextPage} onClick={() => void timeline.fetchNextPage()}>
          Show earlier
        </Button>
      )}
    </div>
  )
}

// who names the people in a conversation, at most three.
function who(interaction: Interaction) {
  const names = interaction.participants.filter((p) => p.role !== 'declined').map((p) => p.name || p.address)
  return names.length > 3 ? `${names.slice(0, 3).join(', ')} +${names.length - 3}` : names.join(', ')
}

// InteractionRow is one conversation in a timeline: its kind, title, people,
// when, and the start of what was said.
export function InteractionRow({ interaction, when }: { interaction: Interaction; when?: string }) {
  return (
    <li>
      <InteractionDialog interactionId={interaction.id}>
        <button
          type="button"
          className="-mx-2.5 flex w-[calc(100%+1.25rem)] gap-3 rounded-[var(--radius-card)] px-2.5 py-2.5 text-left outline-none transition-colors hover:bg-list-hover focus-visible:bg-list-hover"
        >
          <span className="mt-px flex size-7 shrink-0 items-center justify-center rounded-full bg-list-active text-ink-2">
            <KindIcon kind={interaction.kind} />
          </span>
          <span className="min-w-0 flex-1">
            <span className="flex items-baseline gap-3">
              <span className="truncate text-[13px] font-medium text-ink">{interaction.title || 'No subject'}</span>
              <time className="ml-auto shrink-0 text-[12px] tabular-nums text-ink-3" dateTime={interaction.started_at} title={formatDateTime(interaction.started_at)}>
                {when ?? recentOrDate(interaction.started_at)}
              </time>
            </span>
            <span className="block truncate text-[12px] text-ink-3">{interaction.last_message ? <MessageState message={interaction.last_message} /> : who(interaction)}</span>
            {interaction.preview && <span className="mt-1 line-clamp-2 text-[12.5px] leading-[1.5] text-ink-2">{interaction.preview}</span>}
          </span>
        </button>
      </InteractionDialog>
    </li>
  )
}

const kinds: { kind: Kind; label: string }[] = [
  { kind: 'note', label: 'Note' },
  { kind: 'call', label: 'Call' },
  { kind: 'meeting', label: 'Meeting' },
]

// Composer logs a note, call or meeting against a record. It rests as one
// line and opens when written in.
function Composer({ recordId, people }: { recordId: string; people: string[] }) {
  const [kind, setKind] = useState<Kind>('note')
  const [notes, setNotes] = useState('')
  const [open, setOpen] = useState(false)
  const log = useAction<object>('log_interaction')
  const close = () => {
    setNotes('')
    setOpen(false)
  }
  const submit = () => {
    if (notes.trim()) {
      log.mutate({ kind, text: notes.trim(), records: [recordId], people: kind === 'note' ? [] : people }, { onSuccess: close })
    }
  }
  return (
    <div
      className={cn(
        'rounded-[var(--radius-card)] border bg-raised transition-[border-color,box-shadow] duration-150',
        open ? 'border-ink-3/40 shadow-sm' : 'border-border hover:border-ink-3/30',
      )}
    >
      <textarea
        value={notes}
        aria-label="Add a note"
        onFocus={() => setOpen(true)}
        onBlur={() => !notes.trim() && setOpen(false)}
        onChange={(e) => setNotes(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            submit()
          } else if (e.key === 'Escape') {
            close()
            e.currentTarget.blur()
          }
        }}
        rows={open ? 3 : 1}
        placeholder={kind === 'note' ? 'Add a note…' : `What was said on the ${kind}?`}
        className="block w-full resize-none bg-transparent px-3 py-2.5 text-[13px] leading-[1.5] text-ink outline-none placeholder:text-ink-3"
      />
      {open && (
        <div className="flex items-center gap-1 px-2 pb-2" onMouseDown={(e) => e.preventDefault()}>
          {kinds.map((k) => (
            <Button key={k.kind} variant="ghost" aria-pressed={kind === k.kind} onClick={() => setKind(k.kind)} className={kind === k.kind ? 'bg-list-active text-ink' : undefined}>
              <KindIcon kind={k.kind} />
              {k.label}
            </Button>
          ))}
          <span className="ml-auto mr-2 text-[11.5px] text-ink-3">⌘↵</span>
          <Button variant="primary" disabled={!notes.trim() || log.isPending} onClick={submit}>
            Log {kind}
          </Button>
        </div>
      )}
    </div>
  )
}
