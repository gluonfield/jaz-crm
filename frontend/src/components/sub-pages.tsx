import { Button } from '@jaz/ui/button'
import { Link } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { usePages } from '@/lib/pages'
import { useNewPage } from './data-nav'
import { PageIcon } from './page-icon'

// SubPages lists the pages inside a page as links, in the sidebar's order,
// and adds one.
export function SubPages({ page }: { page: string }) {
  const inside = usePages()?.children.get(page) ?? []
  const newPage = useNewPage()
  return (
    <div className="flex flex-col">
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
      <Button variant="ghost" className="-ml-2 mt-1 self-start" onClick={() => newPage(page)}>
        <Plus /> Add page
      </Button>
    </div>
  )
}
