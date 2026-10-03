import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { Row, Section, inputClass } from '@/components/controls'
import { useAction, useWorkspace } from '@/lib/queries'
import { cn } from '@/lib/utils'
import { DeleteWorkspace } from '@/components/delete-workspace'

export const Route = createFileRoute('/_app/settings/')({ component: GeneralPage })

function GeneralPage() {
  const workspace = useWorkspace()
  const admin = workspace?.members?.find((m) => m.is_me)?.admin
  return workspace && (
    <>
      <WorkspaceName key={workspace.id + workspace.name} name={workspace.name} admin={!!admin} />
      {admin && <DeleteWorkspace key={workspace.id + workspace.name} workspace={workspace} />}
    </>
  )
}

function WorkspaceName({ name, admin }: { name: string; admin: boolean }) {
  const [draft, setDraft] = useState(name)
  const update = useAction<{ name: string }>('update_workspace')
  return (
    <Section title="General">
      <Row>
        <label htmlFor="workspace-name" className="w-20 shrink-0 text-ink-2">Name</label>
        <input id="workspace-name" value={draft} disabled={!admin || update.isPending} onChange={(e) => setDraft(e.target.value)} className={cn(inputClass, 'flex-1')} />
        {admin && draft !== name && <Button disabled={!draft.trim() || update.isPending} onClick={() => update.mutate({ name: draft })}>Save</Button>}
      </Row>
    </Section>
  )
}
