import { AlignLeft, Archive, ArrowUpRight, AtSign, Calendar, CircleChevronDown, Contrast, Hash, Link2, Pencil, Phone, SquareCheck, Tags, Trash2, UserRound } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { slugify } from '@/lib/crm'
import { useAction } from '@/lib/queries'
import type { Attribute, AttributeType, CrmObject } from '@/lib/types'
import { cn } from '@/lib/utils'
import { singular } from './create-record'
import { ObjectIcon } from './icons'
import { ConfirmDialog, NameDialog } from './prompt-dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu'
import { menuItem, menuLabel, menuSeparator } from './ui/menu'
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover'

export function ColumnHeader({ object, attribute, icon, className }: { object: CrmObject; attribute: Attribute; icon?: ReactNode; className?: string }) {
  const edit = useAction<object>('edit_attribute')
  const [renaming, setRenaming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          className={cn(
            '-mx-1.5 flex h-7 min-w-0 items-center gap-2 rounded-[var(--radius-control)] px-1.5 font-medium outline-none hover:bg-list-hover hover:text-ink focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:bg-list-active',
            className,
          )}
        >
          {icon}
          <span className="truncate">{attribute.name}</span>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-44">
          <DropdownMenuItem onSelect={() => setRenaming(true)}>
            <Pencil /> Rename…
          </DropdownMenuItem>
          {!attribute.protected && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => edit.mutate({ object: object.slug, attribute: attribute.slug, action: 'archive' })}>
                <Archive /> Archive
              </DropdownMenuItem>
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

type Kind = { label: string; icon: ReactNode; type: AttributeType; multi?: boolean; options?: string[]; target?: string }

const kinds: Kind[] = [
  { label: 'Text', icon: <AlignLeft />, type: 'text' },
  { label: 'Number', icon: <Hash />, type: 'number' },
  { label: 'Select', icon: <CircleChevronDown />, type: 'select' },
  { label: 'Multi-select', icon: <Tags />, type: 'select', multi: true },
  { label: 'Status', icon: <Contrast />, type: 'status', options: ['Not started', 'In progress', 'Done'] },
  { label: 'Date', icon: <Calendar />, type: 'date' },
  { label: 'Person', icon: <UserRound />, type: 'member' },
  { label: 'Checkbox', icon: <SquareCheck />, type: 'checkbox' },
  { label: 'URL', icon: <Link2 />, type: 'url' },
  { label: 'Email', icon: <AtSign />, type: 'email' },
  { label: 'Phone', icon: <Phone />, type: 'phone' },
]

// typeIcon is the icon of a column's type, as the column menu shows it.
export function typeIcon(attribute: Attribute) {
  if (attribute.type === 'reference') {
    return <ArrowUpRight />
  }
  return (kinds.find((k) => k.type === attribute.type && !!k.multi === !!attribute.multi) ?? kinds.find((k) => k.type === attribute.type))?.icon ?? <AlignLeft />
}

// AddColumn names a column and picks its type, or a link to records of
// another object, as Notion adds a property. An unnamed column takes its
// type's name.
export function AddColumn({ object, objects, children }: { object: CrmObject; objects: CrmObject[]; children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const create = useAction<object>('create_attribute')
  const links: Kind[] = objects.filter((o) => o.slug !== 'follow_ups').map((o) => ({ label: o.name, icon: <ObjectIcon slug={o.slug} />, type: 'reference', target: o.slug, multi: true }))
  const add = (kind: Kind) => {
    const label = name.trim() || kind.label
    const base = slugify(label) || slugify(kind.label)
    let n = 1
    while (object.attributes.some((a) => a.slug === (n === 1 ? base : `${base}_${n}`))) {
      n++
    }
    create.mutate({ object: object.slug, slug: n === 1 ? base : `${base}_${n}`, name: n === 1 ? label : `${label} ${n}`, type: kind.type, multi: kind.multi, target: kind.target, options: kind.options })
    setOpen(false)
  }
  const row = (kind: Kind) => (
    <button key={kind.label} type="button" onClick={() => add(kind)} className={cn(menuItem, 'w-full hover:bg-list-hover')}>
      {kind.icon}
      {kind.label}
    </button>
  )
  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        setName('')
      }}
    >
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent align="start" sideOffset={6} className="scrollbar-quiet max-h-[min(75vh,600px)] w-60 overflow-y-auto p-1.5">
        <input
          autoFocus
          aria-label="Column name"
          placeholder="Column name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && add(kinds[0])}
          className="h-8 w-full bg-transparent px-2 text-[13px] text-ink outline-none placeholder:text-ink-3"
        />
        <div className={menuSeparator} />
        <div className={menuLabel}>Type</div>
        {kinds.map(row)}
        <div className={menuSeparator} />
        <div className={menuLabel}>Link to</div>
        {links.map(row)}
      </PopoverContent>
    </Popover>
  )
}
