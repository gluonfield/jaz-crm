import { Button } from '@jaz/ui/button'
import { useRef, useState } from 'react'
import { valueText, valuesOf } from '@/lib/crm'
import { useAction, useWorkspace } from '@/lib/queries'
import type { CrmRecord, DraftMessage, DraftSender } from '@/lib/types'
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from './ui/dialog'
import { HTMLContent } from './html-content'

const list = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText)
const text = (record: CrmRecord, slug: string) => list(record, slug).join(', ')

export function Release({ record, draft, channel, sender, beforeSend, disabled }: { record: CrmRecord; draft: string; channel: string; sender?: DraftSender; beforeSend: (draft: DraftMessage) => Promise<unknown>; disabled?: boolean }) {
  const [open, setOpen] = useState(false)
  const workspace = useWorkspace()
  const state = text(record, 'draft_status')
  const email = channel === 'Email'
  if (state === 'Sending' || (!email && state === 'Approved')) {
    return <span className="shrink-0 py-1.5 pr-1.5 text-[12px] text-ink-3">{state === 'Sending' ? 'Sending…' : 'Approved · waiting for the sender'}</span>
  }
  const unavailable = disabled || !draft.trim() || !channel || (email && (!sender || !sender.subject.trim() || sender.to.length + sender.cc.length === 0))
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild><Button variant="primary" size="sm" disabled={unavailable}>{email ? 'Send' : 'Approve'}</Button></DialogTrigger>
      {open && <DraftConfirmation record={record} draft={draft} sender={sender} channel={channel} workspace={workspace?.name} beforeSend={beforeSend} onClose={() => setOpen(false)} onSent={() => setOpen(false)} />}
    </Dialog>
  )
}

function DraftConfirmation({ record, draft, sender, channel, workspace, beforeSend, onClose, onSent }: { record: CrmRecord; draft: string; sender?: DraftSender; channel: string; workspace?: string; beforeSend: (draft: DraftMessage) => Promise<unknown>; onClose: () => void; onSent: () => void }) {
  const email = channel === 'Email'
  const [prepare] = useState(() => beforeSend)
  // Keep the reviewed text and recipients fixed while background queries refresh.
  const [seen] = useState(() => ({
    record_id: record.id,
    workspace,
    draft: draft.trim(),
    subject: sender?.subject ?? '',
    from: sender?.from,
    to: email ? sender?.to ?? [] : list(record, 'to'),
    cc: email ? sender?.cc ?? [] : list(record, 'cc'),
    signature: sender?.signature,
  }))
  const send = useAction<Omit<typeof seen, 'signature'> & { confirmed: boolean }>('send_draft')
  const sending = useRef(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const pending = saving || send.isPending
  return (
    <DialogContent
      aria-describedby="send-description"
      showCloseButton={false}
      onEscapeKeyDown={(event) => {
        if (pending) {
          event.preventDefault()
        }
      }}
      onInteractOutside={(event) => event.preventDefault()}
      className="max-h-[calc(100dvh-2rem)] overflow-y-auto border-border bg-raised p-4 text-ink"
    >
      <DialogTitle className="text-[14px]">{email ? 'Send email?' : 'Approve reply?'}</DialogTitle>
      <p id="send-description" className="text-[13px] text-ink-2">{email ? 'Send this email now?' : 'Approve this reply for sending?'}</p>
      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[12px] text-ink-2">
        {[['From', seen.from], ['To', seen.to.join(', ')], ['Cc', seen.cc.join(', ')], ['Subject', seen.subject]].filter(([, value]) => value).map(([label, value]) => (
          <div key={label} className="contents"><dt className="text-ink-3">{label}</dt><dd className="min-w-0 [overflow-wrap:anywhere]">{value}</dd></div>
        ))}
      </dl>
      <div className="max-h-[40dvh] overflow-y-auto rounded-[var(--radius-control)] bg-bg p-3 text-[13px] leading-5">
        <p className="whitespace-pre-wrap [overflow-wrap:anywhere]">{seen.draft}</p>
        {seen.signature && <div className="mt-3 border-t border-border pt-2"><HTMLContent html={seen.signature} className="[&_img]:max-h-16" /></div>}
      </div>
      {send.error && <p role="alert" className="text-[13px] text-danger">{send.error.message}</p>}
      {saveError && <p role="alert" className="text-[13px] text-danger">{saveError}</p>}
      <div className="flex justify-end gap-2">
        <Button autoFocus disabled={pending} onClick={onClose}>Cancel</Button>
        <Button variant="primary" disabled={pending} onClick={async () => {
          if (sending.current) {
            return
          }
          sending.current = true
          setSaving(true)
          setSaveError('')
          try {
            await prepare({ draft: seen.draft, subject: seen.subject, to: seen.to, cc: seen.cc })
          } catch (error) {
            setSaveError(error instanceof Error ? error.message : 'Could not save the reply. Try again.')
            sending.current = false
            return
          } finally {
            setSaving(false)
          }
          const { signature: _signature, ...input } = seen
          send.mutate({ ...input, confirmed: true }, {
            onSuccess: onSent,
            onError: () => {
              sending.current = false
            },
          })
        }}>{pending ? (email ? 'Sending…' : 'Approving…') : (email ? 'Send' : 'Approve')}</Button>
      </div>
    </DialogContent>
  )
}
