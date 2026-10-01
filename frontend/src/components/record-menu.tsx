import { useNavigate } from '@tanstack/react-router'
import { ArrowUpRight, Pencil, Tags, Trash2 } from 'lucide-react'
import { type ReactNode, useId, useState } from 'react'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { useAction, useWrite } from '@/lib/queries'
import type { CrmObject, CrmRecord } from '@/lib/types'
import { Button } from './controls'
import { CreateRecord } from './create-record'
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
  const write = useWrite(record)
  const remove = useAction<{ record_id: string }>('delete_record')
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const description = useId()
  const categories = object.attributes.find((a) => a.slug === 'categories' && a.type === 'select')
  const tagged = valuesOf(record, 'categories').map(valueText)
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
        <ContextMenuContent className="w-44">
          <ContextMenuItem onSelect={() => void navigate({ to: '/r/$recordId', params: { recordId: record.id } })}>
            <ArrowUpRight /> Open
          </ContextMenuItem>
          <ContextMenuItem onSelect={() => setEditing(true)}>
            <Pencil /> Edit…
          </ContextMenuItem>
          {categories?.options?.length ? (
            <ContextMenuSub>
              <ContextMenuSubTrigger>
                <Tags /> Categories
              </ContextMenuSubTrigger>
              <ContextMenuSubContent className="w-48">
                {categories.options.map((option) => (
                  <ContextMenuCheckboxItem
                    key={option}
                    checked={tagged.includes(option)}
                    disabled={write.pending}
                    onSelect={(e) => e.preventDefault()}
                    onCheckedChange={(checked) => (checked ? write.set(categories.slug, option) : write.remove(categories.slug, [option]))}
                  >
                    <span className="truncate">{option}</span>
                  </ContextMenuCheckboxItem>
                ))}
              </ContextMenuSubContent>
            </ContextMenuSub>
          ) : null}
          <ContextMenuSeparator />
          <ContextMenuItem variant="destructive" onSelect={() => setDeleting(true)}>
            <Trash2 /> Delete…
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      <CreateRecord object={object} record={record} open={editing} onOpenChange={setEditing} />
      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent aria-describedby={description} showCloseButton={false} className="bg-raised text-ink sm:max-w-sm">
          <DialogTitle className="text-[14px] leading-snug break-words">Delete {recordName(record)}?</DialogTitle>
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
