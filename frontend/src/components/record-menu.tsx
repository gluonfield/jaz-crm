import { useNavigate } from '@tanstack/react-router'
import { ArrowUpRight, Link2, Trash2 } from 'lucide-react'
import { type ReactNode, useId, useState } from 'react'
import { embedded } from '@/lib/api'
import { recordName } from '@/lib/crm'
import { useAction } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { Button } from './controls'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from './ui/context-menu'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'

// RecordMenu offers a record's actions when its row or card is right-clicked.
export function RecordMenu({ record, children }: { record: CrmRecord; children: ReactNode }) {
  const navigate = useNavigate()
  const remove = useAction<{ record_id: string }>('delete_record')
  const [deleting, setDeleting] = useState(false)
  const description = useId()
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
        <ContextMenuContent className="w-44">
          <ContextMenuItem onSelect={() => void navigate({ to: '/r/$recordId', params: { recordId: record.id } })}>
            <ArrowUpRight /> Open
          </ContextMenuItem>
          {!embedded() && (
            <ContextMenuItem onSelect={() => void navigator.clipboard.writeText(`${location.origin}/r/${record.id}`)}>
              <Link2 /> Copy link
            </ContextMenuItem>
          )}
          <ContextMenuSeparator />
          <ContextMenuItem variant="destructive" onSelect={() => setDeleting(true)}>
            <Trash2 /> Delete…
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent aria-describedby={description} showCloseButton={false} className="bg-raised text-ink sm:max-w-sm">
          <DialogTitle className="text-[14px] leading-snug break-words">Delete {recordName(record)}?</DialogTitle>
          <p id={description} className="text-[13px] leading-relaxed text-ink-2">
            Its details and links to conversations will be deleted. This cannot be undone.
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
