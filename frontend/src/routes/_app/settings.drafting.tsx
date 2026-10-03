import { Link, createFileRoute } from '@tanstack/react-router'
import { FileText, Plus, X } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { Row, Section } from '@/components/controls'
import { Picker } from '@/components/picker'
import { recordName } from '@/lib/crm'
import { useDebounced } from '@/lib/hooks'
import { useAction, useRecords, useTool, useWorkspace } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'

export const Route = createFileRoute('/_app/settings/drafting')({ component: DraftingPage })

function DraftingPage() {
  const workspace = useWorkspace()
  const admin = workspace?.members?.find((m) => m.is_me)?.admin
  const update = useAction<{ company_page_ids?: string[]; drafting_web_access?: boolean }>('update_workspace')
  const [search, setSearch] = useState<string>()
  const found = useRecords('pages', useDebounced(search), 50).data?.records ?? []
  const selected = workspace?.company_page_ids ?? []
  const disabled = !admin || update.isPending
  const webAccess = (update.isPending ? update.variables?.drafting_web_access : undefined) ?? workspace?.drafting_web_access ?? false
  const toggle = (id: string) => update.mutate({ company_page_ids: selected.includes(id) ? selected.filter((current) => current !== id) : [...selected, id] })
  return (
    <>
      <Section title="Company knowledge">
        <p className="px-4 py-3 text-[12.5px] text-ink-3">Selected pages and all their child pages are included in full.</p>
        {selected.map((id) => <KnowledgePage key={id} id={id} disabled={disabled} admin={!!admin} onRemove={() => toggle(id)} />)}
        {admin && (
          <Row>
            <Picker trigger={<Button disabled={disabled}><Plus /> Select pages</Button>} placeholder="Find pages…"
              multiple selected={selected} disabled={disabled} onSearch={setSearch} onSelect={toggle}
              options={found.map((page) => ({ value: page.id, label: recordName(page), icon: <FileText className="size-4 shrink-0 text-ink-3" /> }))} />
          </Row>
        )}
      </Section>
      <Section title="Web access">
        <Row>
          <label className="flex min-h-10 flex-1 items-center gap-3">
            <input type="checkbox" checked={webAccess} disabled={disabled || !workspace}
              onChange={(event) => update.mutate({ drafting_web_access: event.target.checked })} />
            <span>
              <span className="block font-medium text-ink">Allow web access</span>
              <span className="block text-[12px] text-ink-3">Search and read public websites when drafting.</span>
            </span>
          </label>
        </Row>
      </Section>
      <p className="text-[12.5px] leading-relaxed text-ink-3">Sender identity, related records and complete stored conversation history are included automatically.</p>
    </>
  )
}

function KnowledgePage({ id, disabled, admin, onRemove }: { id: string; disabled: boolean; admin: boolean; onRemove: () => void }) {
  const { data: page, error } = useTool<CrmRecord>('get_record', { record_id: id })
  const name = page ? recordName(page) : error ? 'Page unavailable' : 'Loading…'
  return (
    <Row>
      <FileText className="size-4 shrink-0 text-ink-3" />
      <Link to="/r/$recordId" params={{ recordId: id }} className="min-w-0 flex-1 truncate text-ink hover:underline">{name}</Link>
      {admin && <Button variant="ghost" disabled={disabled} aria-label={`Remove ${name} from company knowledge`} onClick={onRemove}><X /></Button>}
    </Row>
  )
}
