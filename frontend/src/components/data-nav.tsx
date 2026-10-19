import { useRouterState } from '@tanstack/react-router'
import { ChevronRight, Plus } from 'lucide-react'
import { useState } from 'react'
import { type DataDrag, type Page, type Pages, type Place, isTable, path, useDataDrag, usePages } from '@/lib/pages'
import { readItem, writeItem } from '@/lib/storage'
import type { CrmObject } from '@/lib/types'
import { cn } from '@/lib/utils'
import { ObjectIcon } from './icons'
import { PageIcon } from './page-icon'
import { NavItem } from './nav-item'
import { NewData } from './new-data'
import { PageMenu, TableMenu } from './data-menu'

const openedKey = 'crm-opened-pages'

// Row buttons show on hover, and always on touch screens, where their tap
// area reaches the row's height.
const rowButton =
  'absolute top-1 flex size-5 items-center justify-center rounded-[4px] text-ink-3 opacity-0 outline-none transition-opacity duration-100 hover:bg-list-active hover:text-ink focus-visible:opacity-100 group-hover/row:opacity-100 pointer-coarse:top-2.5 pointer-coarse:opacity-100 pointer-coarse:before:absolute pointer-coarse:before:-inset-2.5 [&_svg]:size-3.5'

// Rows turn off dragging their link, so the browser drags and pictures the row
// itself. lifted's fade is a transition, under reduced motion too, so it starts
// after that picture is taken and the picture stays solid.
const lifted = 'relative rounded-[var(--radius-control)] transition-opacity duration-150 data-[dragging=true]:opacity-50 [&>a]:cursor-grab'

// mark shows where a dragged item would land at a row: tinted inside it, or a
// line above or below it.
const mark = (place?: Place) =>
  cn(
    place === 'inside' && 'bg-primary-soft [&>a]:bg-transparent [&>a]:text-primary',
    place && place !== 'inside' && 'before:absolute before:inset-x-1 before:z-10 before:h-0.5 before:rounded-full before:bg-primary',
    place === 'before' && 'before:-top-px',
    place === 'after' && 'before:-bottom-px',
  )

// DataNav lists the workspace's tables and its pages as a tree, and creates
// either from +. Opened pages stay open, and opening a page opens its branch.
export function DataNav() {
  const pages = usePages()
  const [opened, setOpened] = useState(() => new Set<string>(JSON.parse(readItem(openedKey) ?? '[]')))
  const location = useRouterState({ select: (s) => s.location.pathname })
  const viewing = location.match(/^\/r\/([^/]+)$/)?.[1] ?? (pages && [...pages.children.values()].flat().find((item) => isTable(item) && location === `/o/${item.slug}`)?.parent)
  const branch = `${location}:${viewing}`
  const [revealed, setRevealed] = useState<string>()
  if (pages && viewing && branch !== revealed && pages.byId.has(viewing)) {
    setRevealed(branch)
    setOpened(new Set([...opened, ...path(pages, viewing).map((p) => p.id)]))
  }
  const toggle = (id: string) => {
    const next = new Set(opened)
    if (opened.has(id)) {
      next.delete(id)
    } else {
      next.add(id)
    }
    setOpened(next)
    writeItem(openedKey, JSON.stringify([...next]))
  }
  const drag = useDataDrag(pages, (parent) => {
    if (parent && !opened.has(parent)) {
      toggle(parent)
    }
  })
  return (
    <div className="mt-4 flex flex-col gap-px">
      <div {...drag.onto()} className={cn('flex h-7 items-center rounded-[var(--radius-control)] pl-2 pr-1 text-[12px] font-medium text-ink-3', drag.at() && 'bg-primary-soft text-primary')}>
        <span className="flex-1">{drag.dragging ? 'Move to top level' : 'Data'}</span>
        <NewData>
          <button type="button"
            aria-label="New Page or Database"
            className="relative flex size-5 items-center justify-center rounded-[4px] outline-none hover:bg-list-hover hover:text-ink focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:bg-list-active pointer-coarse:before:absolute pointer-coarse:before:-inset-2.5 [&_svg]:size-3.5"
          >
            <Plus />
          </button>
        </NewData>
      </div>
      {pages && <Items parent="" depth={0} pages={pages} opened={opened} toggle={toggle} drag={drag} />}
    </div>
  )
}

type Tree = { pages: Pages; opened: Set<string>; toggle: (id: string) => void; drag: DataDrag }

function Items({ parent, depth, ...tree }: Tree & { parent: string; depth: number }) {
  return (tree.pages.children.get(parent) ?? []).map((item) =>
    isTable(item) ? <TableItem key={item.slug} object={item} depth={depth} {...tree} /> : <PageItem key={item.id} page={item} depth={depth} {...tree} />,
  )
}

function TableItem({ object, pages, drag, depth }: { object: CrmObject; pages: Pages; drag: DataDrag; depth: number }) {
  return <TableMenu object={object} pages={pages}>
    <div {...drag.from(object)} {...drag.onto(object)} className={cn(lifted, mark(drag.at(object)))}><NavItem to={`/o/${object.slug}`} depth={depth} draggable={false} icon={<ObjectIcon slug={object.slug} icon={object.icon} />}>{object.name}</NavItem></div>
  </TableMenu>
}

function PageItem({ page, depth, ...tree }: Tree & { page: Page; depth: number }) {
  const { pages, opened, toggle, drag } = tree
  const hasChildren = (pages.children.get(page.id) ?? []).length > 0
  const open = opened.has(page.id)
  return (
    <>
      <PageMenu page={page} pages={pages}>
        {/* Below an open page come its children, so a page dropped on its
            bottom edge would land out of sight: it offers only the top. */}
        <div
          {...drag.from(page)}
          {...drag.onto(page, open && hasChildren ? ['before'] : ['before', 'after'])}
          className={cn(lifted, 'group/row pointer-coarse:[&>a]:pr-8', mark(drag.at(page)))}
        >
          <NavItem to={`/r/${page.id}`} depth={depth} draggable={false} icon={<PageIcon value={page.icon} className={cn(hasChildren && 'group-hover/row:invisible pointer-coarse:invisible')} />}>
            {page.name}
          </NavItem>
          {hasChildren && (
            <button type="button" aria-label={`${open ? 'Collapse' : 'Expand'} ${page.name}`} aria-expanded={open} onClick={() => toggle(page.id)} style={{ left: 6 + depth * 14 }} className={rowButton}>
              <ChevronRight className={cn('transition-transform duration-150 motion-reduce:transition-none', open && 'rotate-90')} />
            </button>
          )}
          <NewData parent={page.id}>
            <button type="button" aria-label={`Add inside ${page.name}`} className={cn(rowButton, 'right-1')}><Plus /></button>
          </NewData>
        </div>
      </PageMenu>
      {open && <Items parent={page.id} depth={depth + 1} {...tree} />}
    </>
  )
}
