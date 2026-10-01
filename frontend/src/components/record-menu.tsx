import { useNavigate } from '@tanstack/react-router'
import { ArrowUpRight, Pencil, Plus, Tags, Trash2 } from 'lucide-react'
import { type KeyboardEvent, type ReactNode, useId, useRef, useState } from 'react'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { useAction, useWrite } from '@/lib/queries'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { Button } from './controls'
import { CreateRecord } from './create-record'
import { ValueDot } from './select-field'
import {
  ContextMenu,
  ContextMenuCheckboxItem,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from './ui/context-menu'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'

// RecordMenu offers a record's actions when its row or card is right-clicked.
export function RecordMenu({ object, record, children }: { object: CrmObject; record: CrmRecord; children: ReactNode }) {
  const navigate = useNavigate()
  const remove = useAction<{ record_id: string }>('delete_record', 150)
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const description = useId()
  const categories = object.attributes.find((a) => a.slug === 'categories' && a.type === 'select')
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild data-deleting={remove.isPending || undefined}>
          {children}
        </ContextMenuTrigger>
        {/* Focus returning to the row as the menu closes would pull it out of the dialog an item opens. */}
        <ContextMenuContent className="w-44" onCloseAutoFocus={(e) => e.preventDefault()}>
          <ContextMenuItem onSelect={() => void navigate({ to: '/r/$recordId', params: { recordId: record.id } })}>
            <ArrowUpRight /> Open
          </ContextMenuItem>
          <ContextMenuItem onSelect={() => setEditing(true)}>
            <Pencil /> Edit…
          </ContextMenuItem>
          {categories && <Categories record={record} attribute={categories} />}
          <ContextMenuSeparator />
          <ContextMenuItem onSelect={() => setDeleting(true)}>
            <Trash2 /> Delete…
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      <CreateRecord object={object} record={record} open={editing} onOpenChange={setEditing} />
      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent
          aria-describedby={description}
          showCloseButton={false}
          onOpenAutoFocus={(e) => {
            const dialog = e.currentTarget as HTMLElement
            e.preventDefault()
            dialog.focus()
          }}
          className="bg-raised text-ink sm:max-w-sm"
        >
          <DialogTitle className="text-[14px] leading-snug [overflow-wrap:anywhere]">Delete {recordName(record)}?</DialogTitle>
          <p id={description} className="text-[13px] leading-relaxed text-ink-2">
            Its details and links to conversations will be deleted. This cannot be undone.
            {record.object === 'people' && ' Their addresses move to Skipped in Triage, so sync will not add them back.'}
            {record.object === 'companies' && ' Contacts at its domains move to Skipped in Triage, including future senders, so sync will not add the company back.'}
          </p>
          <div className="flex justify-end gap-2">
            <Button onClick={() => setDeleting(false)}>Cancel</Button>
            <Button
              className="border-danger/30 bg-danger/10 text-danger hover:bg-danger/20"
              onClick={() => {
                setDeleting(false)
                remove.mutate({ record_id: record.id })
              }}
            >
              Delete
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}

// Categories tags a record the way Linear labels an issue: type to filter, tick
// to toggle, and Enter on a new name to create it and tag the record with it.
function Categories({ record, attribute }: { record: CrmRecord; attribute: Attribute }) {
  const write = useWrite(record)
  const add = useAction<{ object: string; attribute: string; value: string }, { value: string }>('add_attribute_option')
  const [query, setQuery] = useState('')
  const input = useRef<HTMLInputElement>(null)
  const tagged = valuesOf(record, attribute.slug).map(valueText)
  const text = query.trim()
  const shown = (attribute.options ?? []).filter((o) => o.toLowerCase().includes(text.toLowerCase()))
  const exact = shown.find((o) => o.toLowerCase() === text.toLowerCase())
  const toggle = (option: string) => (tagged.includes(option) ? write.remove(attribute.slug, [option]) : write.set(attribute.slug, option))
  const create = () => add.mutate({ object: record.object, attribute: attribute.slug, value: text }, { onSuccess: (out) => write.set(attribute.slug, out.value) })
  // Typing anywhere in the submenu, or on its row, goes to the filter.
  const typeIn = (e: KeyboardEvent) => {
    if (e.key.length === 1 && e.key !== ' ' && !e.metaKey && !e.ctrlKey && !e.altKey && e.target !== input.current) {
      e.stopPropagation()
      input.current?.focus()
    }
  }
  const choose = () => {
    if (exact) {
      toggle(exact)
    } else {
      create()
    }
    setQuery('')
  }
  return (
    <ContextMenuSub>
      <ContextMenuSubTrigger onKeyDownCapture={typeIn}>
        <Tags /> Categories
      </ContextMenuSubTrigger>
      <ContextMenuSubContent className="w-56" onKeyDownCapture={typeIn}>
        <input
          ref={input}
          value={query}
          aria-label="Add categories"
          placeholder="Add categories…"
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key !== 'Escape') {
              e.stopPropagation()
            }
            if (e.key === 'Enter' && text) {
              choose()
            }
            if (e.key === 'ArrowDown') {
              e.currentTarget.parentElement?.querySelector<HTMLElement>('[role^=menuitem]')?.focus()
            }
          }}
          className="h-8 w-full bg-transparent px-2 text-[13px] text-ink outline-none placeholder:text-ink-3"
        />
        {(shown.length > 0 || (text && !exact)) && <ContextMenuSeparator />}
        {shown.map((option) => (
          <ContextMenuCheckboxItem key={option} checked={tagged.includes(option)} disabled={write.pending} onSelect={(e) => e.preventDefault()} onCheckedChange={() => toggle(option)}>
            <ValueDot value={option} />
            <span className="truncate">{option}</span>
          </ContextMenuCheckboxItem>
        ))}
        {text && !exact && (
          <ContextMenuItem
            onSelect={(e) => {
              e.preventDefault()
              choose()
            }}
          >
            <Plus /> Create “{text}”
          </ContextMenuItem>
        )}
      </ContextMenuSubContent>
    </ContextMenuSub>
  )
}
