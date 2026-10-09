import { Link } from '@tanstack/react-router'
import { CalendarClock, Video } from 'lucide-react'
import type { ReactNode } from 'react'
import { joinURL, valueText, valuesOf } from '@/lib/crm'
import { formatDate, formatNumber, timeAgo } from '@/lib/format'
import { useWorkspace } from '@/lib/queries'
import type { CrmObject, CrmRecord, Interaction, Member, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { DomainLink } from './domain-link'
import { ExternalLink } from './external-link'
import { RecordIcon } from './icons'
import { Stage } from './stage'
import { InteractionDialog } from './interaction-dialog'

const when = new Intl.DateTimeFormat('en', { weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })

// order puts a record's identity in reading order: where it stands, what it
// is worth, what it does, who it belongs to, then how to reach it.
const order = ['status', 'number', 'text', 'reference', 'member', 'domain', 'email']

// identity is one line of a record's short, single facts.
function identity(record: CrmRecord, object: CrmObject, members: Member[]) {
  const facts: ReactNode[] = []
  const draft = object.slug === 'follow_ups' ? ['draft', 'to', 'cc'] : []
  const attributes = object.attributes.filter((a) => !['name', 'context', 'notes', ...draft].includes(a.slug) && order.includes(a.type))
  attributes.sort((a, b) => order.indexOf(a.type) - order.indexOf(b.type))
  for (const a of attributes) {
    const values = valuesOf(record, a.slug)
    const first = values[0]
    if (!first) {
      continue
    }
    const text = valueText(first)
    if (a.type === 'status') {
      facts.push(<Stage key={a.slug} stage={text} />)
    } else if (a.type === 'number') {
      facts.push(<span key={a.slug}>{formatNumber(text, a.slug)}</span>)
    } else if (a.type === 'text' && !a.multi && text.length <= 60) {
      facts.push(<span key={a.slug}>{text}</span>)
    } else if (a.type === 'reference' && !a.multi) {
      const ref = first as Ref
      facts.push(
        <Link key={a.slug} to="/r/$recordId" params={{ recordId: ref.id }} className="inline-flex items-center gap-1.5 text-ink hover:underline">
          <RecordIcon object={a.target ?? ''} name={ref.name ?? ''} photo={ref.photo} icon={ref.icon} size={16} />
          {ref.name}
        </Link>,
      )
    } else if (a.type === 'domain') {
      facts.push(
        ...values.map((v) => (
          <DomainLink key={valueText(v)} domain={valueText(v)} className="text-ink" />
        )),
      )
    } else if (a.type === 'member') {
      const member = members.find((m) => m.email === text)
      // A teammate is named with the field, so an owner never reads as a relationship.
      facts.push(
        <span key={a.slug} className="inline-flex items-center gap-1.5">
          <span className="text-ink-3">{a.name}</span>
          <RecordIcon object="people" name={member?.name || text} photo={member?.photo} size={16} />
          {member?.name || text}
        </span>,
      )
    } else if (a.type === 'email') {
      facts.push(<span key={a.slug}>{text}</span>)
    }
  }
  return facts
}

// Summary says who a record is and where things stand: its facts in a line,
// its conversations so far, and what is scheduled next.
export function Summary({ record, object, name, upcoming, children }: { record: CrmRecord; object: CrmObject; name: string; upcoming: Interaction[]; children: ReactNode }) {
  const facts = identity(record, object, useWorkspace()?.members ?? [])
  const activity = record.activity
  const next = upcoming[0]
  const join = next && joinURL(next)
  const touch = activity?.interactions
    ? [`Last contact ${timeAgo(activity.last_at ?? record.created_at)}`, `${activity.interactions} conversation${activity.interactions === 1 ? '' : 's'} since ${formatDate(activity.first_at ?? record.created_at)}`]
    : []
  return (
    <div className="flex items-start gap-4">
      <RecordIcon object={record.object} name={name} photo={record.photo} size={48} className="mt-0.5" />
      <div className="min-w-0 flex-1">
        {children}
        {facts.length > 0 && <Line className="mt-1 text-[13px] text-ink-2">{facts}</Line>}
        {(touch.length > 0 || next) && (
          <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1.5 text-[12.5px] text-ink-3">
            {touch.length > 0 && <Line>{touch}</Line>}
            {join && (
              <ExternalLink href={join} className="inline-flex h-6 items-center gap-1.5 rounded-full bg-primary px-2.5 font-medium text-on-primary no-underline hover:bg-primary-strong hover:text-on-primary hover:no-underline">
                <Video className="size-3.5" /> Join
              </ExternalLink>
            )}
            {next && (
              <InteractionDialog interactionId={next.id}>
                <button
                  type="button"
                  className="inline-flex h-6 items-center gap-1.5 rounded-full bg-primary-soft px-2.5 text-ink outline-none transition-colors hover:bg-primary/25 focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <CalendarClock className="size-3.5 text-primary" />
                  <span className="font-medium">{when.format(new Date(next.started_at))}</span>
                  <span className="max-w-60 truncate text-ink-2">{next.title}</span>
                </button>
              </InteractionDialog>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

// Line runs items together with a dot between each.
function Line({ className, children }: { className?: string; children: ReactNode[] }) {
  return (
    <p className={cn('flex flex-wrap items-center gap-x-2 gap-y-1', className)}>
      {children.map((item, i) => (
        <span key={i} className="inline-flex items-center gap-2">
          {i > 0 && <span className="text-ink-3">·</span>}
          {item}
        </span>
      ))}
    </p>
  )
}
