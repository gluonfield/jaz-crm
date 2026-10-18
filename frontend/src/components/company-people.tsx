import { Link } from '@tanstack/react-router'
import { recordName } from '@/lib/crm'
import type { CrmRecord } from '@/lib/types'
import { RecordStack } from './controls'

const link = 'flex h-7 min-w-0 items-center rounded-[var(--radius-control)] px-1 text-ink-2 outline-none hover:bg-list-hover hover:text-ink focus-visible:bg-list-hover'

// CompanyPeople is a company's people on one line, opening the one person or
// the list of them.
export function CompanyPeople({ company }: { company: CrmRecord }) {
  const people = company.related?.['people.company'] ?? []
  const stack = <RecordStack object="people" refs={people} />
  return (
    <div className="flex min-w-0" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()} onContextMenu={(e) => e.stopPropagation()}>
      {people.length === 1 && <Link to="/r/$recordId" params={{ recordId: people[0].id }} className={link}>{stack}</Link>}
      {people.length > 1 && (
        <Link to="/o/$object" params={{ object: 'people' }} search={{ filters: [{ attribute: 'company', operator: 'is', value: company.id }] }} aria-label={`People at ${recordName(company)}`} className={link}>
          {stack}
        </Link>
      )}
    </div>
  )
}
