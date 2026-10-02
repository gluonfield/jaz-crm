import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { DraftConfirmation, reviewInBrowser } from '@/components/draft-release'
import { Dialog } from '@/components/ui/dialog'
import { embedded } from '@/lib/api'
import { valueText, valuesOf } from '@/lib/crm'
import { useTool } from '@/lib/queries'
import type { CrmRecord, DraftSender } from '@/lib/types'

export const Route = createFileRoute('/send/$recordId')({
  ssr: false,
  validateSearch: (search: Record<string, unknown>): { workspace?: string } => ({ workspace: typeof search.workspace === 'string' ? search.workspace : undefined }),
  component: SendPage,
})

function SendPage() {
  const { recordId } = Route.useParams()
  const { workspace } = Route.useSearch()
  const [result, setResult] = useState('')
  const record = useTool<CrmRecord>('get_record', { record_id: recordId, workspace }, { enabled: !embedded(), retry: false })
  const field = (slug: string) => record.data ? valuesOf(record.data, slug).map(valueText).join(', ') : ''
  const channel = field('channel')
  const email = channel === 'Email'
  const sender = useTool<DraftSender>('get_draft_sender', { record_id: recordId, workspace }, { enabled: !embedded() && email, retry: false })
  const error = record.error?.message ?? sender.error?.message
  const status = field('draft_status')
  const closed = ['Sending', 'Sent', ...(!email ? ['Approved'] : [])].includes(status)
  const outcome = status === 'Sent' ? 'Email sent.' : status === 'Approved' ? 'Reply approved.' : status
  const unavailable = record.data && (record.data.object !== 'follow_ups' || !field('draft').trim() || !channel)
  return (
    <main className="flex h-full flex-col items-center justify-center gap-3 bg-bg p-4 text-[13px] text-ink">
      {embedded() ? <Button variant="primary" onClick={() => void reviewInBrowser(recordId, workspace ?? '')}>Review in browser</Button> : <>
        <p role={error ? 'alert' : 'status'}>{result || error || (closed ? outcome : unavailable ? 'This follow-up has no reply ready to send.' : 'Loading reply…')}</p>
        {(result || error || closed || unavailable) && <Button onClick={() => window.location.assign('/o/follow_ups')}>Back to CRM</Button>}
        {!result && !error && !closed && !unavailable && record.data && (!email || sender.data) && (
          <Dialog open onOpenChange={(open) => {
            if (!open) {
              setResult('Cancelled. Nothing sent.')
            }
          }}>
            <DraftConfirmation record={record.data} sender={sender.data} channel={channel} workspace={workspace} onClose={() => setResult('Cancelled. Nothing sent.')} onSent={() => setResult(email ? 'Email sent.' : 'Reply approved.')} />
          </Dialog>
        )}
      </>}
    </main>
  )
}
