import { Link } from '@tanstack/react-router'
import { useQueries } from '@tanstack/react-query'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { toolQuery } from '@/lib/queries'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { RecordIcon } from './icons'

// detail is the first plain value worth showing beside a related record's
// name, such as a person's job title or a deal's stage.
function detail(record: CrmRecord, object: CrmObject) {
  const attribute = object.attributes.find((a) => a.slug !== 'name' && ['text', 'select', 'number', 'date'].includes(a.type) && valuesOf(record, a.slug).length > 0)
  return attribute ? valueText(valuesOf(record, attribute.slug)[0]) : ''
}

// Related lists the records that point at this one, grouped by object and
// attribute: the people at a company, its deals.
export function Related({ recordId, object, objects }: { recordId: string; object: string; objects: CrmObject[] }) {
  const links = objects.flatMap((o) => o.attributes.filter((a) => a.type === 'reference' && a.target === object).map((a) => ({ object: o, attribute: a })))
  const results = useQueries({
    queries: links.map(({ object: o, attribute: a }) => toolQuery<{ records: CrmRecord[] }>('search_records', { object: o.slug, where: { [a.slug]: recordId }, limit: 50 })),
  })
  return links.map(({ object: o, attribute: a }, i) => {
    const records = results[i].data?.records ?? []
    if (records.length === 0) {
      return null
    }
    return (
      <section key={`${o.slug}.${a.slug}`} className="mt-8">
        <h2 className="mb-2 flex items-baseline gap-2 text-[13px] font-medium text-ink">
          {title(o, a, links.filter((l) => l.object.slug === o.slug).length)}
          <span className="font-normal tabular-nums text-ink-3">{records.length}</span>
        </h2>
        <ul className="-mx-3 flex flex-col gap-0.5">
          {records.map((r) => (
            <li key={r.id}>
              <Link to="/r/$recordId" params={{ recordId: r.id }} className="flex h-9 items-center gap-2.5 rounded-[var(--radius-control)] px-3 text-[13px] hover:bg-list-hover">
                <RecordIcon object={o.slug} name={recordName(r)} photo={r.photo} size={20} />
                <span className="truncate font-medium text-ink">{recordName(r)}</span>
                <span className="truncate text-ink-3">{detail(r, o)}</span>
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
