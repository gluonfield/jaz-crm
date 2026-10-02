import { useNavigate, useRouterState } from '@tanstack/react-router'
import { ChevronRight, FileText, FolderInput, Pencil, Plus, Table2, Trash2 } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { slugify } from '@/lib/crm'
import { type Page, type Pages, path, usePages, within } from '@/lib/pages'
import { useAction, useObjects } from '@/lib/queries'
import { readItem, writeItem } from '@/lib/storage'
import type { CrmObject, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'
import { ObjectIcon } from './icons'
import { NavItem } from './nav-item'
import { ConfirmDialog, NameDialog } from './prompt-dialog'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuOptions, ContextMenuSeparator, ContextMenuTrigger } from './ui/context-menu'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './ui/dropdown-menu'

const openedKey = 'crm-opened-pages'

const rowButton =
  'absolute top-1 flex size-5 items-center justify-center rounded-[4px] text-ink-3 opacity-0 outline-none transition-opacity duration-100 hover:bg-list-active hover:text-ink focus-visible:opacity-100 group-hover/row:opacity-100 [&_svg]:size-3.5'

// useNewPage creates an untitled page, inside a parent when given, and opens it.
export function useNewPage() {
  const navigate = useNavigate()
  const create = useAction<object, { record: CrmRecord }>('upsert_record')
  return (parent?: string) =>
    create.mutate(
      { object: 'pages', values: parent ? { name: 'Untitled', parent } : { name: 'Untitled' } },
      { onSuccess: (out) => void navigate({ to: '/r/$recordId', params: { recordId: out.record.id } }) },
    )
}

// DataNav lists the workspace's tables and its pages as a tree, and creates
// either from +. Opened pages stay open, and opening a page opens its branch.
export function DataNav() {
  const tables = (useObjects() ?? []).filter((o) => !o.standard)
  const pages = usePages()
  const navigate = useNavigate()
  const newPage = useNewPage()
  const createTable = useAction<object, { slug: string }>('create_object')
  const [naming, setNaming] = useState(false)
  const [opened, setOpened] = useState(() => new Set<string>(JSON.parse(readItem(openedKey) ?? '[]')))
  const viewing = useRouterState({ select: (s) => s.location.pathname }).match(/^\/r\/([^/]+)$/)?.[1]
  const [revealed, setRevealed] = useState<string>()
  if (pages && viewing && viewing !== revealed && pages.byId.has(viewing)) {
    setRevealed(viewing)
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
  return (
    <div className="mt-4 flex flex-col gap-px">
      <div className="flex h-7 items-center pl-2 pr-1 text-[12px] font-medium text-ink-3">
        <span className="flex-1">Data</span>
        <DropdownMenu>
          <DropdownMenuTrigger
            aria-label="New page or table"
            className="flex size-5 items-center justify-center rounded-[4px] outline-none hover:bg-list-hover hover:text-ink focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:bg-list-active [&_svg]:size-3.5"
          >
            <Plus />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-44">
            <DropdownMenuItem onSelect={() => newPage()}>
              <FileText /> Page
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setNaming(true)}>
              <Table2 /> Table
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {tables.map((o) => (
        <TableItem key={o.slug} object={o} />
      ))}
      {pages?.children.get('')?.map((page) => <PageItem key={page.id} page={page} pages={pages} depth={0} opened={opened} toggle={toggle} />)}
      <NameDialog
        open={naming}
        onOpenChange={setNaming}
        title="New table"
        placeholder="Name, such as Painpoints"
        action="Create"
        onSubmit={(name) => createTable.mutate({ slug: slugify(name), name }, { onSuccess: (o) => void navigate({ to: '/o/$object', params: { object: o.slug } }) })}
      />
    </div>
  )
}

function TableItem({ object }: { object: CrmObject }) {
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
            <NavItem to={`/o/${object.slug}`} icon={<ObjectIcon slug={object.slug} />}>
              {object.name}
            </NavItem>
          </div>
        </ContextMenuTrigger>
        <ContextMenuContent className="w-48" onCloseAutoFocus={(e) => e.preventDefault()}>
          <ContextMenuItem onSelect={() => setRenaming(true)}>
            <Pencil /> Rename…
          </ContextMenuItem>
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

function PageItem({ page, pages, depth, opened, toggle }: { page: Page; pages: Pages; depth: number; opened: Set<string>; toggle: (id: string) => void }) {
  const inside = pages.children.get(page.id) ?? []
  const open = opened.has(page.id)
  const newPage = useNewPage()
  return (
    <>
      <PageMenu page={page} pages={pages}>
        <div className="group/row relative">
          <NavItem to={`/r/${page.id}`} depth={depth} icon={<FileText className={cn(inside.length > 0 && 'group-hover/row:invisible')} />}>
            {page.name}
          </NavItem>
          {inside.length > 0 && (
            <button type="button" aria-label={`${open ? 'Collapse' : 'Expand'} ${page.name}`} aria-expanded={open} onClick={() => toggle(page.id)} style={{ left: 6 + depth * 14 }} className={rowButton}>
              <ChevronRight className={cn('transition-transform duration-150 motion-reduce:transition-none', open && 'rotate-90')} />
            </button>
          )}
          <button type="button" aria-label={`Add a page inside ${page.name}`} onClick={() => newPage(page.id)} className={cn(rowButton, 'right-1')}>
            <Plus />
          </button>
        </div>
      </PageMenu>
      {open && inside.map((child) => <PageItem key={child.id} page={child} pages={pages} depth={depth + 1} opened={opened} toggle={toggle} />)}
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
  const places = [
    { value: '', label: 'Top level' },
    ...[...pages.byId.values()]
      .filter((p) => !within(pages, p.id, page.id))
      .map((p) => ({ value: p.id, label: path(pages, p.id).map((a) => a.name).join(' / '), icon: <FileText /> })),
  ]
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
        <ContextMenuContent className="w-52" onCloseAutoFocus={(e) => e.preventDefault()}>
          <ContextMenuItem onSelect={() => newPage(page.id)}>
            <Plus /> Add page inside
          </ContextMenuItem>
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
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${page.name}?`}
        onConfirm={() =>
          remove.mutate({ record_id: page.id }, { onSuccess: () => viewing && void navigate(page.parent ? { to: '/r/$recordId', params: { recordId: page.parent } } : { to: '/' }) })
        }
      >
        Its content will be deleted, and the pages inside it move to the top level. This cannot be undone.
      </ConfirmDialog>
    </>
  )
}
