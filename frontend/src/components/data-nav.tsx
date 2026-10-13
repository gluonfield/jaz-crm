import { useNavigate, useRouterState } from '@tanstack/react-router'
import { ChevronRight, FileText, FolderInput, Pencil, Plus, Table2, Trash2 } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { type Page, type PageDrag, type Pages, path, usePageDrag, usePages, within } from '@/lib/pages'
import { useAction } from '@/lib/queries'
import { readItem, writeItem } from '@/lib/storage'
import type { CrmObject } from '@/lib/types'
import { cn } from '@/lib/utils'
import { ObjectIcon } from './icons'
import { PageIcon } from './page-icon'
import { NavItem } from './nav-item'
import { NewData, NewTable, useNewPage } from './new-data'
import { ConfirmDialog, NameDialog } from './prompt-dialog'
import { RecordDeleteDialog } from './record-delete-dialog'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuOptions, ContextMenuSeparator, ContextMenuTrigger } from './ui/context-menu'

const openedKey = 'crm-opened-pages'

// Row buttons show on hover, and always on touch screens, where their tap
// area reaches the row's height.
const rowButton =
  'absolute top-1 flex size-5 items-center justify-center rounded-[4px] text-ink-3 opacity-0 outline-none transition-opacity duration-100 hover:bg-list-active hover:text-ink focus-visible:opacity-100 group-hover/row:opacity-100 pointer-coarse:top-2.5 pointer-coarse:opacity-100 pointer-coarse:before:absolute pointer-coarse:before:-inset-2.5 [&_svg]:size-3.5'

// DataNav lists the workspace's tables and its pages as a tree, and creates
// either from +. Opened pages stay open, and opening a page opens its branch.
export function DataNav() {
  const pages = usePages()
  const [opened, setOpened] = useState(() => new Set<string>(JSON.parse(readItem(openedKey) ?? '[]')))
  const location = useRouterState({ select: (s) => s.location.pathname })
  const viewing = location.match(/^\/r\/([^/]+)$/)?.[1] ?? (pages && [...pages.tables.values()].flat().find((table) => location === `/o/${table.slug}`)?.parent)
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
  const drag = usePageDrag(pages, (parent) => {
    if (parent && !opened.has(parent)) {
      toggle(parent)
    }
  })
  return (
    <div className="mt-4 flex flex-col gap-px">
      <div {...drag.onto('')} className={cn('flex h-7 items-center rounded-[var(--radius-control)] pl-2 pr-1 text-[12px] font-medium text-ink-3', drag.target === '' && 'bg-primary-soft text-primary')}>
        <span className="flex-1">{drag.dragging ? 'Move to top level' : 'Data'}</span>
        <NewData>
          <button type="button"
            aria-label="New page or table"
            className="relative flex size-5 items-center justify-center rounded-[4px] outline-none hover:bg-list-hover hover:text-ink focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:bg-list-active pointer-coarse:before:absolute pointer-coarse:before:-inset-2.5 [&_svg]:size-3.5"
          >
            <Plus />
          </button>
        </NewData>
      </div>
      {pages?.tables.get('')?.map((table) => <TableItem key={table.slug} object={table} pages={pages} />)}
      {pages?.children.get('')?.map((page) => <PageItem key={page.id} page={page} pages={pages} depth={0} opened={opened} toggle={toggle} drag={drag} />)}
    </div>
  )
}

function TableItem({ object, pages, depth = 0 }: { object: CrmObject; pages: Pages; depth?: number }) {
  const navigate = useNavigate()
  const viewing = useRouterState({ select: (s) => s.location.pathname }) === `/o/${object.slug}`
  const edit = useAction<object>('edit_object')
  const [renaming, setRenaming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild>
          <div>
            <NavItem to={`/o/${object.slug}`} depth={depth} icon={<ObjectIcon slug={object.slug} />}>
              {object.name}
            </NavItem>
          </div>
        </ContextMenuTrigger>
        <ContextMenuContent className="w-48" onCloseAutoFocus={(e) => e.preventDefault()}>
          <ContextMenuItem onSelect={() => setRenaming(true)}>
            <Pencil /> Rename…
          </ContextMenuItem>
          <ContextMenuOptions
            icon={<FolderInput />}
            label="Move to"
            placeholder="Move to…"
            options={[
              { value: '', label: 'Top level' },
              ...[...pages.byId.values()].map((page) => ({ value: page.id, label: path(pages, page.id).map((ancestor) => ancestor.name).join(' / '), icon: <PageIcon value={page.icon} /> })),
            ]}
            selected={[object.parent ?? '']}
            onSelect={(parent) => edit.mutate({ object: object.slug, action: 'move', parent })}
          />
          <ContextMenuSeparator />
          <ContextMenuItem onSelect={() => setDeleting(true)}>
            <Trash2 /> Delete…
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      <NameDialog open={renaming} onOpenChange={setRenaming} title="Rename table" initial={object.name} action="Rename" onSubmit={(name) => edit.mutate({ object: object.slug, action: 'rename', name })} />
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${object.name}?`}
        onConfirm={() => edit.mutate({ object: object.slug, action: 'delete' }, { onSuccess: () => viewing && void navigate({ to: '/' }) })}
      >
        Its records and their content will be deleted, with every column that references them. This cannot be undone.
      </ConfirmDialog>
    </>
  )
}

function PageItem({ page, pages, depth, opened, toggle, drag }: { page: Page; pages: Pages; depth: number; opened: Set<string>; toggle: (id: string) => void; drag: PageDrag }) {
  const inside = pages.children.get(page.id) ?? []
  const tables = pages.tables.get(page.id) ?? []
  const hasChildren = inside.length + tables.length > 0
  const open = opened.has(page.id)
  return (
    <>
      <PageMenu page={page} pages={pages}>
        <div {...drag.from(page.id)} {...drag.onto(page.id)} className={cn('group/row relative rounded-[var(--radius-control)] [&>a]:cursor-grab pointer-coarse:[&>a]:pr-8', drag.dragging === page.id && 'opacity-50', drag.target === page.id && 'bg-primary-soft [&>a]:bg-transparent [&>a]:text-primary')}>
          <NavItem to={`/r/${page.id}`} depth={depth} icon={<PageIcon value={page.icon} className={cn(hasChildren && 'group-hover/row:invisible pointer-coarse:invisible')} />}>
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
      {open && <>
        {tables.map((table) => <TableItem key={table.slug} object={table} pages={pages} depth={depth + 1} />)}
        {inside.map((child) => <PageItem key={child.id} page={child} pages={pages} depth={depth + 1} opened={opened} toggle={toggle} drag={drag} />)}
      </>}
    </>
  )
}

// PageMenu adds a page inside one, moves it under another page or to the top
// level, and deletes it; its pages then move to the top level.
function PageMenu({ page, pages, children }: { page: Page; pages: Pages; children: ReactNode }) {
  const navigate = useNavigate()
  const viewing = useRouterState({ select: (s) => s.location.pathname }) === `/r/${page.id}`
  const newPage = useNewPage()
  const write = useAction<object>('upsert_record')
  const remove = useAction<{ record_id: string }>('delete_record')
  const [deleting, setDeleting] = useState(false)
  const [naming, setNaming] = useState(false)
  const places = [
    { value: '', label: 'Top level' },
    ...[...pages.byId.values()]
      .filter((p) => !within(pages, p.id, page.id))
      .map((p) => ({ value: p.id, label: path(pages, p.id).map((a) => a.name).join(' / '), icon: <PageIcon value={p.icon} /> })),
  ]
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
        <ContextMenuContent className="w-52" onCloseAutoFocus={(e) => e.preventDefault()}>
          <ContextMenuItem onSelect={() => newPage(page.id)}>
            <FileText /> Add page inside
          </ContextMenuItem>
          <ContextMenuItem onSelect={() => setNaming(true)}><Table2 /> Add table inside</ContextMenuItem>
          <ContextMenuOptions
            icon={<FolderInput />}
            label="Move to"
            placeholder="Move to…"
            options={places}
            selected={[page.parent ?? '']}
            onSelect={(parent) => write.mutate({ object: 'pages', record_id: page.id, ...(parent ? { values: { parent } } : { remove: { parent: [] } }) })}
          />
          <ContextMenuSeparator />
          <ContextMenuItem onSelect={() => setDeleting(true)}>
            <Trash2 /> Delete…
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      <NewTable parent={page.id} open={naming} onOpenChange={setNaming} />
      <RecordDeleteDialog
        open={deleting}
        onOpenChange={setDeleting}
        name={page.name}
        object="pages"
        onConfirm={() =>
          remove.mutate({ record_id: page.id }, { onSuccess: () => viewing && void navigate(page.parent ? { to: '/r/$recordId', params: { recordId: page.parent } } : { to: '/' }) })
        }
      />
    </>
  )
}
