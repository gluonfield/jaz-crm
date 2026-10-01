import { Link } from '@tanstack/react-router'
import { recordName } from '@/lib/crm'
import type { CrmRecord } from '@/lib/types'
import { RecordIcon } from './icons'

export function CompanyPeople({ company }: { company: CrmRecord }) {
  const people = company.related?.['people.company'] ?? []
  return (
    <div className="flex max-w-96 items-center gap-1 py-1" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()} onContextMenu={(e) => e.stopPropagation()}>
      {people.slice(0, 3).map((person) => (
        <Link key={person.id} to="/r/$recordId" params={{ recordId: person.id }} title={person.name || 'Unnamed'}
          className="flex h-7 min-w-0 max-w-32 items-center gap-1.5 rounded-[var(--radius-control)] px-1 outline-none hover:bg-list-hover hover:text-ink focus-visible:bg-list-hover">
          <RecordIcon object="people" name={person.name || 'Unnamed'} photo={person.photo} />
          <span className="truncate">{person.name || 'Unnamed'}</span>
        </Link>
      ))}
      {people.length > 3 && <Link to="/o/$object" params={{ object: 'people' }} search={{ filters: [{ attribute: 'company', operator: 'is', value: company.id }] }}
        aria-label={`More people at ${recordName(company)}`} className="flex h-7 shrink-0 items-center rounded-[var(--radius-control)] px-1 text-[12px] text-ink-3 outline-none hover:bg-list-hover hover:text-ink focus-visible:bg-list-hover">More</Link>}
    </div>
  )
}
