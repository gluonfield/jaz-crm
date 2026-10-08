import type { ComponentProps } from 'react'
import { ConfirmDialog } from './prompt-dialog'

export function RecordDeleteDialog({ name, object, ...dialog }: Omit<ComponentProps<typeof ConfirmDialog>, 'title' | 'children'> & { name: string; object: string }) {
  return (
    <ConfirmDialog {...dialog} title={`Delete ${name}?`}>
      You can restore it from Trash.
      {object === 'pages' && ' Pages inside it remain available at the top level.'}
      {object === 'people' && ' Their addresses move to Skipped in Triage until restored.'}
      {object === 'companies' && ' Contacts at its domains move to Skipped in Triage until restored.'}
    </ConfirmDialog>
  )
}
