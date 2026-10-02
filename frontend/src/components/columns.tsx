import { Button } from '@jaz/ui/button'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useAction } from '@/lib/queries'
import type { Attribute, CrmObject } from '@/lib/types'
import { singular } from './create-record'
import { NewAttribute } from './new-attribute'
import { ConfirmDialog, NameDialog } from './prompt-dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu'
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover'

// ColumnHeader names a table's column and renames or deletes it; the name
// column stays.
export function ColumnHeader({ object, attribute }: { object: CrmObject; attribute: Attribute }) {
  const edit = useAction<object>('edit_attribute')
  const [renaming, setRenaming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger className="-mx-1.5 flex h-7 items-center rounded-[var(--radius-control)] px-1.5 font-medium outline-none hover:bg-list-hover hover:text-ink focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:bg-list-active">
          {attribute.name}
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-44">
          <DropdownMenuItem onSelect={() => setRenaming(true)}>
            <Pencil /> Rename…
          </DropdownMenuItem>
          {attribute.slug !== 'name' && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => setDeleting(true)}>
                <Trash2 /> Delete…
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <NameDialog open={renaming} onOpenChange={setRenaming} title="Rename column" initial={attribute.name} action="Rename" onSubmit={(name) => edit.mutate({ object: object.slug, attribute: attribute.slug, action: 'rename', name })} />
      <ConfirmDialog open={deleting} onOpenChange={setDeleting} title={`Delete ${attribute.name}?`} onConfirm={() => edit.mutate({ object: object.slug, attribute: attribute.slug, action: 'delete' })}>
        Its value in every {singular(object)} will be deleted, with the saved views filtering by it. This cannot be undone.
      </ConfirmDialog>
    </>
  )
}

// AddColumn adds a column of any type, such as a reference to companies.
export function AddColumn({ object, objects }: { object: CrmObject; objects: CrmObject[] }) {
  const [open, setOpen] = useState(false)
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label="Add column">
          <Plus />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" sideOffset={6} className="w-[440px] p-3">
        <NewAttribute object={object} objects={objects} onDone={() => setOpen(false)} />
      </PopoverContent>
    </Popover>
  )
}
