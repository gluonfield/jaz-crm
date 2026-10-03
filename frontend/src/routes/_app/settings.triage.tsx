import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { Row, Section, inputClass } from '@/components/controls'
import { useAction, useWorkspace } from '@/lib/queries'
import { cn } from '@/lib/utils'
import { TriageSettings } from '@/components/triage-settings'

export const Route = createFileRoute('/_app/settings/triage')({ component: TriagePage })

function TriagePage() {
  const workspace = useWorkspace()
  const admin = !!workspace?.members?.find((m) => m.is_me)?.admin
  return (
    <>
      {workspace && <Criteria key={workspace.id + workspace.description} description={workspace.description} admin={admin} />}
      <TriageSettings admin={admin} />
    </>
  )
}

function Criteria({ description, admin }: { description: string; admin: boolean }) {
  const [draft, setDraft] = useState(description)
  const update = useAction<{ description: string }>('update_workspace')
  return (
    <Section title="Who belongs">
      <Row>
        <textarea aria-label="Who belongs" value={draft} disabled={!admin || update.isPending} rows={3}
          onChange={(e) => setDraft(e.target.value)} placeholder="Customers, suppliers and partners of our press shop"
          className={cn(inputClass, 'h-auto flex-1 resize-y py-1.5 leading-[1.5]')} />
      </Row>
      {admin && draft !== description && <Row className="justify-end"><Button disabled={update.isPending} onClick={() => update.mutate({ description: draft })}>Save</Button></Row>}
    </Section>
  )
}
