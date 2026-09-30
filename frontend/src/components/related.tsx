import { Link } from '@tanstack/react-router'
import { useQueries } from '@tanstack/react-query'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { timeAgo } from '@/lib/format'
import { toolQuery } from '@/lib/queries'
import { statusOf } from '@/lib/stages'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { RecordIcon } from './icons'
import { Stage } from './stage'

// detail is the first short plain value worth showing beside a related
// record's name, such as a person's job title.
function detail(record: CrmRecord, object: CrmObject) {
  const attribute = object.attributes.find((a) => a.slug !== 'name' && ['text', 'select', 'number', 'date'].includes(a.type) && valuesOf(record, a.slug).length > 0)
  const text = attribute ? valueText(valuesOf(record, attribute.slug)[0]) : ''
  return text.length > 60 ? '' : text
}

const lastContact = (r: CrmRecord) => r.activity?.last_at ?? ''

// Related lists the records that point at this one, grouped by object and
// attribute: the people at a company, its deals. People come most recently
// in touch first.
export function Related({ recordId, object, objects }: { recordId: string; object: string; objects: CrmObject[] }) {
  const links = objects.flatMap((o) => o.attributes.filter((a) => a.type === 'reference' && a.target === object).map((a) => ({ object: o, attribute: a })))
  const results = useQueries({
    queries: links.map(({ object: o, attribute: a }) => toolQuery<{ records: CrmRecord[] }>('search_records', { object: o.slug, where: { [a.slug]: recordId }, limit: 50 })),
  })
  return links.map(({ object: o, attribute: a }, i) => {
    const records = [...(results[i].data?.records ?? [])].sort((x, y) => lastContact(y).localeCompare(lastContact(x)))
    if (records.length === 0) {
      return null
    }
    const status = statusOf(o.attributes)
    return (
      <section key={`${o.slug}.${a.slug}`}>
        <h2 className="mb-1 flex items-baseline gap-2 text-[12px] font-medium text-ink-3">
          {title(o, a, links.filter((l) => l.object.slug === o.slug).length)}
          <span className="tabular-nums">{records.length}</span>
        </h2>
        <ul className="grid gap-x-6 @3xl:grid-cols-2">
          {records.map((r) => (
            <li key={r.id} className="min-w-0">
              <Link
                to="/r/$recordId"
                params={{ recordId: r.id }}
                className="-mx-2 flex h-11 items-center gap-2.5 rounded-[var(--radius-control)] px-2 text-[13px] outline-none transition-colors hover:bg-list-hover focus-visible:bg-list-hover"
              >
                <RecordIcon object={o.slug} name={recordName(r)} photo={r.photo} size={26} />
                <span className="flex min-w-0 flex-col">
                  <span className="truncate font-medium leading-tight text-ink">{recordName(r)}</span>
                  <span className="truncate text-[12px] leading-tight text-ink-3">
                    {status && valuesOf(r, status.slug)[0] ? <Stage attribute={status} stage={valueText(valuesOf(r, status.slug)[0])} /> : detail(r, o)}
                  </span>
                </span>
                {r.activity?.last_at && <span className="ml-auto shrink-0 pl-2 text-[12px] tabular-nums text-ink-3">{timeAgo(r.activity.last_at)}</span>}
              </Link>
            </li>
          ))}
        </ul>
      </section>
    )
  })
}

// title names a group by its object, adding the attribute when one object
// points here in more than one way.
function title(object: CrmObject, attribute: Attribute, ways: number) {
  return ways > 1 ? `${object.name} · ${attribute.name}` : object.name
}
