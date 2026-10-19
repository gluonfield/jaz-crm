import { useNavigate, useRouterState } from '@tanstack/react-router'
import { ArrowUpRight, FileText, FolderInput, Pencil, Table2, Trash2 } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { type Item, type Page, type Pages, isTable, path, useMove, within } from '@/lib/pages'
import { useAction } from '@/lib/queries'
import type { CrmObject } from '@/lib/types'
import { RecordCopyItems } from './record-copy-items'
import { PageIcon } from './page-icon'
import { NewTable, useNewPage } from './new-data'
import { ConfirmDialog, NameDialog } from './prompt-dialog'
import { RecordDeleteDialog } from './record-delete-dialog'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuOptions, ContextMenuSeparator, ContextMenuTrigger } from './ui/context-menu'

export function TableMenu({ object, pages, children }: { object: CrmObject; pages: Pages; children: ReactNode }) {
  const navigate = useNavigate()
  const viewing = useRouterState({ select: (s) => s.location.pathname }) === `/o/${object.slug}`
  const edit = useAction<object>('edit_object')
  const [renaming, setRenaming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild>
          {children}
        </ContextMenuTrigger>
        <ContextMenuContent className="w-48" onCloseAutoFocus={(e) => e.preventDefault()}>
          <ContextMenuItem onSelect={() => void navigate({ to: '/o/$object', params: { object: object.slug } })}><ArrowUpRight /> Open</ContextMenuItem>
          <ContextMenuItem onSelect={() => setRenaming(true)}>
            <Pencil /> Rename…
          </ContextMenuItem>
          <MoveTo item={object} pages={pages} />
          <ContextMenuSeparator />
          <ContextMenuItem onSelect={() => setDeleting(true)}>
            <Trash2 /> Delete…
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      <NameDialog open={renaming} onOpenChange={setRenaming} title="Rename Database" initial={object.name} action="Rename" onSubmit={(name) => edit.mutate({ object: object.slug, action: 'rename', name })} />
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${object.name}?`}
        onConfirm={() => edit.mutate({ object: object.slug, action: 'delete' }, { onSuccess: () => viewing && void navigate(object.parent ? { to: '/r/$recordId', params: { recordId: object.parent } } : { to: '/' }) })}
      >
        Its records and their content will be deleted, with every column that references them. This cannot be undone.
      </ConfirmDialog>
    </>
  )
}

export function PageMenu({ page, pages, children }: { page: Page; pages: Pages; children: ReactNode }) {
  const navigate = useNavigate()
  const viewing = useRouterState({ select: (s) => s.location.pathname }) === `/r/${page.id}`
  const newPage = useNewPage()
  const write = useAction<object>('upsert_record')
  const remove = useAction<{ record_id: string }>('delete_record')
  const [deleting, setDeleting] = useState(false)
  const [naming, setNaming] = useState(false)
  const [renaming, setRenaming] = useState(false)
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
        <ContextMenuContent className="w-52" onCloseAutoFocus={(e) => e.preventDefault()}>
          <ContextMenuItem onSelect={() => void navigate({ to: '/r/$recordId', params: { recordId: page.id } })}><ArrowUpRight /> Open</ContextMenuItem>
          <ContextMenuItem onSelect={() => setRenaming(true)}><Pencil /> Rename…</ContextMenuItem>
          <ContextMenuSeparator />
          <ContextMenuItem onSelect={() => newPage(page.id)}>
            <FileText /> Add Page
          </ContextMenuItem>
          <ContextMenuItem onSelect={() => setNaming(true)}><Table2 /> Add Database</ContextMenuItem>
          <MoveTo item={page} pages={pages} />
          <RecordCopyItems id={page.id} context />
          <ContextMenuSeparator />
          <ContextMenuItem onSelect={() => setDeleting(true)}>
            <Trash2 /> Delete…
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      <NewTable parent={page.id} open={naming} onOpenChange={setNaming} />
      <NameDialog open={renaming} onOpenChange={setRenaming} title="Rename Page" initial={page.name} action="Rename" onSubmit={(name) => write.mutate({ object: 'pages', record_id: page.id, values: { name } })} />
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

// MoveTo moves a page or table inside another page, or to the top level.
function MoveTo({ item, pages }: { item: Item; pages: Pages }) {
  const { move } = useMove()
  return (
    <ContextMenuOptions
      icon={<FolderInput />}
      label="Move to"
      placeholder="Move to…"
      options={[
        { value: '', label: 'Top level' },
        ...[...pages.byId.values()]
          .filter((p) => isTable(item) || !within(pages, p.id, item.id))
          .map((p) => ({ value: p.id, label: path(pages, p.id).map((a) => a.name).join(' / '), icon: <PageIcon value={p.icon} /> })),
      ]}
      selected={[item.parent ?? '']}
      onSelect={(parent) => move(pages, item, { place: 'inside', page: pages.byId.get(parent) })}
    />
  )
}
