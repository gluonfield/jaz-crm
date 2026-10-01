import { useNavigate } from '@tanstack/react-router'
import { ArrowUpRight, Contrast, Pencil, Tags, Trash2, UserRound } from 'lucide-react'
import { type ReactNode, useId, useState } from 'react'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { useAction, useWorkspace, useWrite } from '@/lib/queries'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { Button } from './controls'
import { CreateRecord } from './create-record'
import { RecordIcon } from './icons'
import { ValueDot } from './select-field'
import { StageDot } from './stage'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuOptions, ContextMenuSeparator, ContextMenuTrigger } from './ui/context-menu'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'

// Choice attributes are set from the menu, each under its kind's icon.
const propertyIcons: Partial<Record<Attribute['type'], ReactNode>> = { status: <Contrast />, member: <UserRound />, select: <Tags /> }

// RecordMenu offers a record's actions when its row or card is right-clicked.
export function RecordMenu({ object, record, children }: { object: CrmObject; record: CrmRecord; children: ReactNode }) {
  const navigate = useNavigate()
  const remove = useAction<{ record_id: string }>('delete_record', 150)
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const description = useId()
  const properties = object.attributes.filter((a) => propertyIcons[a.type])
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild data-deleting={remove.isPending || undefined}>
          {children}
        </ContextMenuTrigger>
        {/* Focus returning to the row as the menu closes would pull it out of the dialog an item opens. */}
        <ContextMenuContent className="w-52" onCloseAutoFocus={(e) => e.preventDefault()}>
          {properties.map((attribute) => (
            <Property key={attribute.slug} record={record} attribute={attribute} />
          ))}
          {properties.length > 0 && <ContextMenuSeparator />}
          <ContextMenuItem onSelect={() => void navigate({ to: '/r/$recordId', params: { recordId: record.id } })}>
            <ArrowUpRight /> Open
          </ContextMenuItem>
          <ContextMenuItem onSelect={() => setEditing(true)}>
            <Pencil /> Edit…
          </ContextMenuItem>
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

// Property sets a choice attribute, as Linear's menu sets an issue's status,
// assignee or labels.
function Property({ record, attribute }: { record: CrmRecord; attribute: Attribute }) {
  const write = useWrite(record)
  const add = useAction<{ object: string; attribute: string; value: string }, { value: string }>('add_attribute_option')
  const members = useWorkspace()?.members ?? []
  const values = valuesOf(record, attribute.slug).map(valueText)
  const options =
    attribute.type === 'member'
      ? members.map((m) => ({ value: m.email, label: m.name || m.email, icon: <RecordIcon object="people" name={m.name || m.email} photo={m.photo} size={16} /> }))
      : (attribute.options ?? []).map((value) => ({ value, label: value, icon: attribute.type === 'status' ? <StageDot stage={value} /> : <ValueDot value={value} /> }))
  const select = attribute.type === 'select'
  return (
    <ContextMenuOptions
      icon={propertyIcons[attribute.type]}
      label={attribute.name}
      placeholder={select ? `Add ${attribute.name.toLowerCase()}…` : undefined}
      options={options}
      selected={values}
      multiple={attribute.multi}
      disabled={write.pending || add.isPending}
      onSelect={(value) => (select && values.includes(value) ? write.remove(attribute.slug, [value]) : write.set(attribute.slug, value))}
      onCreate={select ? (value) => add.mutate({ object: record.object, attribute: attribute.slug, value }, { onSuccess: (out) => write.set(attribute.slug, out.value) }) : undefined}
    />
  )
}
