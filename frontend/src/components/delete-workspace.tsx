import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useId, useState } from 'react'
import { call } from '@/lib/api'
import type { Workspace } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Button, Row, Section, inputClass } from './controls'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'

export function DeleteWorkspace({ workspace }: { workspace: Workspace }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const description = useId()
  const client = useQueryClient()
  const navigate = useNavigate()
  const confirmed = name.trim() === workspace.name
  const changeOpen = (next: boolean) => {
    if (!pending) {
      setOpen(next)
      setName('')
      setError('')
    }
  }
  return (
    <>
      <Section title="Delete workspace">
        <Row className="flex-wrap">
          <p className="min-w-48 flex-1 text-ink-2">Permanently delete this workspace and its CRM data for everyone.</p>
          <Button className="border-danger/30 text-danger hover:bg-danger/10" onClick={() => changeOpen(true)}>Delete workspace</Button>
        </Row>
      </Section>
      <Dialog open={open} onOpenChange={changeOpen}>
        <DialogContent aria-describedby={description} className="bg-raised text-ink">
          <DialogTitle>Delete workspace?</DialogTitle>
          <p id={description} className="text-[13px] leading-relaxed text-ink-2">
            All records, conversations, members, invitations, connections and API keys in this workspace will be deleted. Everyone will lose access. This cannot be undone. Emails in Google and your other workspaces are kept.
          </p>
          <form className="flex flex-col gap-4" onSubmit={async (event) => {
            event.preventDefault()
            if (pending || !confirmed) {
              return
            }
            setPending(true)
            setError('')
            try {
              await call('delete_workspace', { workspace_id: workspace.id, name })
              await client.cancelQueries()
              await navigate({ to: '/workspace-deleted' })
              client.clear()
            } catch (failure) {
              setError(failure instanceof Error ? failure.message : 'Could not delete workspace')
            } finally {
              setPending(false)
            }
          }}>
            <label className="flex flex-col gap-2 text-[13px] text-ink-2">
              <span>Type <strong className="break-words font-medium text-ink">{workspace.name}</strong> to confirm</span>
              <input autoFocus autoComplete="off" value={name} disabled={pending} onChange={(event) => setName(event.target.value)} className={cn(inputClass, 'h-8')} />
            </label>
            {error && <p role="alert" className="text-[13px] text-danger">{error}</p>}
            <div className="flex justify-end gap-2">
              <Button disabled={pending} onClick={() => changeOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={pending || !confirmed} className="border-danger/30 bg-danger/10 text-danger hover:bg-danger/20">
                {pending ? 'Deleting…' : 'Delete workspace'}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}
