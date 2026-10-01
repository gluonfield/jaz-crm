import { Row, Section } from '@/components/controls'
import { useAction, useTool } from '@/lib/queries'
import type { TriageSettings as Settings } from '@/lib/types'

const rules: { key: keyof Settings; label: string; detail: string }[] = [
  { key: 'auto_keep_email', label: 'People we email', detail: 'Email threads where someone in this workspace sent a message, with at most 10 participants.' },
  { key: 'auto_keep_meetings', label: 'People we meet', detail: 'Calendar meetings with at most 10 participants. Declined attendees stay pending.' },
  { key: 'auto_keep_records', label: 'Existing CRM records', detail: 'Addresses already attached to a CRM record.' },
  { key: 'auto_keep_ai', label: 'AI approvals', detail: 'Use the workspace’s “Who belongs” description. Requires an AI classifier configured on the server.' },
]

export function TriageSettings({ admin }: { admin: boolean }) {
  const { data } = useTool<Settings>('get_triage_settings')
  const update = useAction<Settings>('update_triage_settings')
  return (
    <Section id="triage" title="Triage">
      <p className="px-4 py-3 text-[12.5px] leading-relaxed text-ink-3">Contacts wait for manual approval unless a rule below is enabled. Rules apply to pending contacts across the workspace. Address and domain decisions made in Triage still apply.</p>
      {rules.map(({ key, label, detail }) => (
        <Row key={key}>
          <label className="flex flex-1 items-center gap-3">
            <input type="checkbox" checked={data?.[key] ?? false} disabled={!admin || !data || update.isPending} onChange={(event) => data && update.mutate({ ...data, [key]: event.target.checked })} />
            <span>
              <span className="block font-medium text-ink">{label}</span>
              <span className="block text-[12px] text-ink-3">{detail}</span>
            </span>
          </label>
        </Row>
      ))}
    </Section>
  )
}
