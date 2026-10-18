import { Link } from '@tanstack/react-router'
import { CalendarClock, Video } from 'lucide-react'
import type { ReactNode } from 'react'
import { joinURL, valueText, valuesOf } from '@/lib/crm'
import { formatDate, formatNumber, timeAgo } from '@/lib/format'
import { useWorkspace } from '@/lib/queries'
import type { Attribute, CrmObject, CrmRecord, Interaction, Member, Ref, Value } from '@/lib/types'
import { cn } from '@/lib/utils'
import { DomainLink, ExternalLink } from './external-link'
import { RecordIcon } from './icons'
import { Stage } from './stage'
import { InteractionDialog } from './interaction-dialog'

const when = new Intl.DateTimeFormat('en', { weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })

// order puts a record's identity in reading order: where it stands, what it
// is worth, what it does, who it belongs to, then how to reach it.
const order = ['status', 'number', 'text', 'reference', 'domain', 'email']

// relationship says whether an attribute names someone the record is with:
// a teammate, or a single linked person.
const relationship = (a: Attribute) => a.type === 'member' || (a.type === 'reference' && !a.multi && a.target === 'people')

// identity is one line of a record's short, single facts; the people it is
// with show on cards beneath.
function identity(record: CrmRecord, object: CrmObject, members: Member[]) {
  const facts: ReactNode[] = []
  const draft = object.slug === 'follow_ups' ? ['draft', 'to', 'cc'] : []
  const attributes = object.attributes.filter((a) => !['name', 'context', ...draft].includes(a.slug) && order.includes(a.type) && !relationship(a))
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
      facts.push(<Person key={a.slug} attribute={a} value={first} members={members} />)
    } else if (a.type === 'domain') {
      facts.push(
        ...values.map((v) => (
          <DomainLink key={valueText(v)} value={valueText(v)} className="text-ink" />
        )),
      )
    } else if (a.type === 'email') {
      facts.push(<span key={a.slug}>{text}</span>)
    }
  }
  return facts
}

// Summary says who a record is and where things stand: its facts in a line,
// then cards for the people it is with and when we last spoke, and what is
// scheduled next.
export function Summary({ record, object, name, upcoming, children }: { record: CrmRecord; object: CrmObject; name: string; upcoming: Interaction[]; children: ReactNode }) {
  const members = useWorkspace()?.members ?? []
  const facts = identity(record, object, members)
  const activity = record.activity
  const next = upcoming[0]
  const join = next && joinURL(next)
  const meeting = next && (
    <>
      {join && (
        <ExternalLink href={join} className="inline-flex h-6 items-center gap-1.5 rounded-full bg-primary px-2.5 font-medium text-on-primary no-underline hover:bg-primary-strong hover:text-on-primary hover:no-underline">
          <Video className="size-3.5" /> Join
        </ExternalLink>
      )}
      <InteractionDialog interactionId={next.id}>
        <button
          type="button"
          className="inline-flex h-6 items-center gap-1.5 rounded-full bg-bg/60 px-2.5 text-ink outline-none transition-colors hover:bg-bg focus-visible:ring-2 focus-visible:ring-ring"
        >
          <CalendarClock className="size-3.5 text-primary" />
          <span className="font-medium">{when.format(new Date(next.started_at))}</span>
          <span className="max-w-60 truncate text-ink-2">{next.title}</span>
        </button>
      </InteractionDialog>
    </>
  )
  return (
    <>
      <div className="flex items-center gap-5">
        <RecordIcon object={record.object} name={name} photo={record.photo} size={56} />
        <div className="min-w-0 flex-1">
          {children}
          {facts.length > 0 && <Line className="mt-1 text-[14px] text-ink-2">{facts}</Line>}
        </div>
      </div>
      <div className="mt-6 flex flex-col gap-3">
        <div className="grid gap-2.5 @lg:grid-cols-3">
          {object.attributes.filter(relationship).flatMap((a) => {
            const value = valuesOf(record, a.slug)[0]
            return value ? [<Card key={a.slug} label={a.name}><Person attribute={a} value={value} members={members} size={22} /></Card>] : []
          })}
          <Card label="Last contact">
            {activity?.interactions ? (
              <span className="truncate">
                {timeAgo(activity.last_at ?? record.created_at)}
                <span className="font-normal text-ink-3"> · {activity.interactions} conversation{activity.interactions === 1 ? '' : 's'} since {formatDate(activity.first_at ?? record.created_at)}</span>
              </span>
            ) : <span className="font-normal text-ink-2">Not in touch yet</span>}
          </Card>
        </div>
        {meeting && (
          <div className="flex flex-wrap items-center gap-3 rounded-[var(--radius-card)] bg-primary-soft px-4 py-3 text-[13px]">
            <span className="font-medium">Next</span>
            {meeting}
          </div>
        )}
      </div>
    </>
  )
}

// Card is one fact about a relationship, set apart on a raised fill.
function Card({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0 rounded-[12px] bg-surface px-3.5 py-3">
      <div className="mb-1.5 truncate text-[12px] text-ink-3">{label}</div>
      <div className="flex min-h-[22px] items-center truncate text-[13.5px] font-medium text-ink">{children}</div>
    </div>
  )
}

// Person is a teammate or a linked record, as its avatar and name.
function Person({ attribute, value, members, size = 16 }: { attribute: Attribute; value: Value; members: Member[]; size?: number }) {
  if (attribute.type === 'member') {
    const text = valueText(value)
    const member = members.find((m) => m.email === text)
    return (
      <span className="inline-flex min-w-0 items-center gap-1.5">
        <RecordIcon object="people" name={member?.name || text} photo={member?.photo} size={size} />
        <span className="truncate">{member?.name || text}</span>
      </span>
    )
  }
  const ref = value as Ref
  return (
    <Link to="/r/$recordId" params={{ recordId: ref.id }} className="inline-flex min-w-0 items-center gap-1.5 text-ink hover:underline">
      <RecordIcon object={attribute.target ?? ''} name={ref.name ?? ''} photo={ref.photo} icon={ref.icon} size={size} />
      <span className="truncate">{ref.name}</span>
    </Link>
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
