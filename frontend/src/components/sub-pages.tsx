import { Button } from '@jaz/ui/button'
import { Link } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { usePages } from '@/lib/pages'
import { ObjectIcon } from './icons'
import { NewData } from './new-data'
import { PageIcon } from './page-icon'

export function SubPages({ page }: { page: string }) {
  const pages = usePages()
  const inside = pages?.children.get(page) ?? []
  const tables = pages?.tables.get(page) ?? []
  return (
    <div className="flex flex-col">
      {tables.map((table) => <Link key={table.slug} to="/o/$object" params={{ object: table.slug }} className="-mx-2 flex h-8 items-center gap-2 rounded-[var(--radius-control)] px-2 text-[15px] text-ink outline-none transition-colors duration-100 hover:bg-list-hover focus-visible:bg-list-hover pointer-coarse:h-10">
        <ObjectIcon slug={table.slug} className="size-4 shrink-0 text-ink-2" />
        <span className="truncate underline decoration-ink-3/50 underline-offset-[3px]">{table.name}</span>
      </Link>)}
      {inside.map((p) => (
        <Link
          key={p.id}
          to="/r/$recordId"
          params={{ recordId: p.id }}
          className="-mx-2 flex h-8 items-center gap-2 rounded-[var(--radius-control)] px-2 text-[15px] text-ink outline-none transition-colors duration-100 hover:bg-list-hover focus-visible:bg-list-hover pointer-coarse:h-10"
        >
          <PageIcon value={p.icon} />
          <span className="truncate underline decoration-ink-3/50 underline-offset-[3px]">{p.name}</span>
        </Link>
      ))}
      <NewData parent={page}>
        <Button variant="ghost" className="-ml-2 mt-1 self-start"><Plus /> Add inside</Button>
      </NewData>
    </div>
  )
}
