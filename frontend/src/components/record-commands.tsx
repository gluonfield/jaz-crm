import { useNavigate } from '@tanstack/react-router'
import { useRef, useState } from 'react'
import { recordName } from '@/lib/crm'
import { copyRecord } from '@/lib/record-copy'
import { useAction, useWorkspace, useWrite } from '@/lib/queries'
import { statusShortcutIds, type ShortcutActions } from '@/lib/shortcut-definitions'
import { useShortcutActions } from '@/lib/shortcuts'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { CreateRecord } from './create-record'
import { Field } from './fields'
import { RecordDeleteDialog } from './record-delete-dialog'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'

export function RecordCommands({ record, object, page = false, onDelete }: { record: CrmRecord; object: CrmObject; page?: boolean; onDelete?: () => void }) {
  const navigate = useNavigate()
  const write = useWrite(record)
  const remove = useAction<object>('delete_record')
  const saveDate = useAction<object>('save_action_date')
  const me = useWorkspace()?.members?.find((member) => member.is_me)
  const [form, setForm] = useState<'create' | 'edit'>()
  const [deleting, setDeleting] = useState(false)
  const [property, setProperty] = useState<Attribute>()
  const field = useRef<HTMLDivElement>(null)
  const status = object.attributes.find((attribute) => attribute.type === 'status')
  const priority = object.attributes.find((attribute) => attribute.slug === 'priority')
  const estimate = object.attributes.find((attribute) => attribute.slug === 'estimate')
  const owner = object.attributes.find((attribute) => attribute.type === 'member')
  const due = object.attributes.find((attribute) => ['due_date', 'action_date'].includes(attribute.slug))
  const labels = object.attributes.find((attribute) => attribute.slug === 'tags' && attribute.type === 'select')
  const name = recordName(record)
  const editName = () => {
    const input = page && document.querySelector<HTMLTextAreaElement>('textarea[aria-label="Name"]')
    if (input) {
      input.focus()
      input.select()
    } else {
      setForm('edit')
    }
  }
  const statusActions: ShortcutActions = {}
  status?.options?.slice(0, 9).forEach((value, index) => {
    statusActions[statusShortcutIds[index]] = () => write.set(status.slug, value)
  })
  useShortcutActions({
    ...statusActions,
    create: () => setForm('create'),
    copyId: () => { void copyRecord(record.id, 'copyId') },
    copyUrl: () => { void copyRecord(record.id, 'copyUrl') },
    edit: () => {
      const editor = page && document.querySelector<HTMLElement>('[contenteditable=true]')
      if (editor) {
        editor.focus()
      } else {
        setForm('edit')
      }
    },
    rename: editName,
    deleteRecord: onDelete ?? (() => setDeleting(true)),
    ...(status && { status: () => setProperty(status) }),
    ...(priority && { priority: () => setProperty(priority) }),
    ...(estimate && { estimate: () => setProperty(estimate) }),
    ...(owner && { assignee: () => setProperty(owner) }),
    ...(owner && me && { assignMe: () => write.set(owner.slug, me.email) }),
    ...(due && { dueDate: () => setProperty(due), clearDueDate: () => record.object === 'follow_ups' && due.slug === 'action_date' ? saveDate.mutate({ record_id: record.id, value: '' }) : write.remove(due.slug, []) }),
    ...(labels && { labels: () => setProperty(labels), removeLabel: () => setProperty(labels) }),
  }, 2)
  return <>
    <CreateRecord object={object} record={form === 'edit' ? record : undefined} open={!!form} openCreated={form === 'create'} onOpenChange={(open) => {
      if (!open) {
        setForm(undefined)
      }
    }} />
    <RecordDeleteDialog open={deleting} onOpenChange={setDeleting} name={name} object={record.object} onConfirm={() => remove.mutate({ record_id: record.id }, {
      onSuccess: () => {
        if (page) {
          void navigate({ to: '/o/$object', params: { object: object.slug } })
        }
      },
    })} />
    <Dialog open={!!property} onOpenChange={(open) => {
      if (!open) {
        setProperty(undefined)
      }
    }}>
      <DialogContent className="max-w-sm" onOpenAutoFocus={(event) => {
        const control = field.current?.querySelector<HTMLElement>('button, input, textarea')
        if (control) {
          event.preventDefault()
          control.focus()
          if (control instanceof HTMLButtonElement) {
            control.click()
          }
        }
      }}>
        <DialogTitle>{property?.name}</DialogTitle>
        {property && <div ref={field}><Field record={record} attribute={property} onCommit={property.multi ? undefined : () => setProperty(undefined)} /></div>}
      </DialogContent>
    </Dialog>
  </>
}
