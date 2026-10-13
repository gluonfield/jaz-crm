import { Button } from '@jaz/ui/button'
import { Link } from '@tanstack/react-router'
import { FileText, Table2 } from 'lucide-react'
import { useState } from 'react'
import { usePages } from '@/lib/pages'
import { ObjectIcon } from './icons'
import { PageMenu, TableMenu } from './data-menu'
import { NewTable, useNewPage } from './new-data'
import { PageIcon } from './page-icon'

export function SubPages({ page }: { page: string }) {
  const pages = usePages()
  const inside = pages?.children.get(page) ?? []
  const tables = pages?.tables.get(page) ?? []
  const newPage = useNewPage()
  const [naming, setNaming] = useState(false)
  return (
    <div className="flex flex-col">
      {pages && tables.map((table) => <TableMenu key={table.slug} object={table} pages={pages}><Link to="/o/$object" params={{ object: table.slug }} className="-mx-2 flex h-8 items-center gap-2 rounded-[var(--radius-control)] px-2 text-[15px] text-ink outline-none transition-colors duration-100 hover:bg-list-hover focus-visible:bg-list-hover pointer-coarse:h-10">
        <ObjectIcon slug={table.slug} icon={table.icon} className="size-4 shrink-0 text-ink-2" />
        <span className="truncate underline decoration-ink-3/50 underline-offset-[3px]">{table.name}</span>
      </Link></TableMenu>)}
      {pages && inside.map((p) => (
        <PageMenu key={p.id} page={p} pages={pages}>
        <Link
          to="/r/$recordId"
          params={{ recordId: p.id }}
          className="-mx-2 flex h-8 items-center gap-2 rounded-[var(--radius-control)] px-2 text-[15px] text-ink outline-none transition-colors duration-100 hover:bg-list-hover focus-visible:bg-list-hover pointer-coarse:h-10"
        >
          <PageIcon value={p.icon} />
          <span className="truncate underline decoration-ink-3/50 underline-offset-[3px]">{p.name}</span>
        </Link>
        </PageMenu>
      ))}
      <div className="-ml-2 mt-1 flex flex-wrap gap-1">
        <Button variant="ghost" onClick={() => newPage(page)}><FileText /> Add Page</Button>
        <Button variant="ghost" onClick={() => setNaming(true)}><Table2 /> Add Database</Button>
      </div>
      <NewTable parent={page} open={naming} onOpenChange={setNaming} />
    </div>
  )
}
