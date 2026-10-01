import { Link } from '@tanstack/react-router'
import { recordName } from '@/lib/crm'
import { useTool } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { RecordIcon } from './icons'

export function CompanyPeople({ company }: { company: CrmRecord }) {
  const result = useTool<{ records: CrmRecord[] }>('search_records', { object: 'people', where: { company: company.id }, limit: 4 })
  if (result.isPending) {
    return <span aria-label="Loading people" className="inline-block h-4 w-16 animate-pulse rounded-[var(--radius-control)] bg-list-hover" />
  }
  if (result.isError) {
    return <span className="text-ink-3" title={result.error.message}>Unavailable</span>
  }
  const people = result.data.records
  return (
    <div className="flex max-w-96 items-center gap-1 py-1" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()} onContextMenu={(e) => e.stopPropagation()}>
      {people.slice(0, 3).map((person) => (
        <Link key={person.id} to="/r/$recordId" params={{ recordId: person.id }} title={recordName(person)}
          className="flex h-7 min-w-0 max-w-32 items-center gap-1.5 rounded-[var(--radius-control)] px-1 outline-none hover:bg-list-hover hover:text-ink focus-visible:bg-list-hover">
          <RecordIcon object="people" name={recordName(person)} photo={person.photo} />
          <span className="truncate">{recordName(person)}</span>
        </Link>
      ))}
      {people.length > 3 && <Link to="/o/$object" params={{ object: 'people' }} search={{ filters: [{ attribute: 'company', operator: 'is', value: company.id }] }}
        aria-label={`More people at ${recordName(company)}`} className="flex h-7 shrink-0 items-center rounded-[var(--radius-control)] px-1 text-[12px] text-ink-3 outline-none hover:bg-list-hover hover:text-ink focus-visible:bg-list-hover">More</Link>}
    </div>
  )
}
