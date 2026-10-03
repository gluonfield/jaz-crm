import { useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { LoaderCircle } from 'lucide-react'
import { call } from '@/lib/api'
import { valueText, valuesOf } from '@/lib/crm'
import type { CrmRecord, DraftMessage, DraftSender, Interaction, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Release } from './draft-release'

const list = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText)
const text = (record: CrmRecord, slug: string) => list(record, slug).join(', ')
const drafted = { drafting: 'Drafting a message…', completed: 'Message drafted', failed: 'Couldn’t draft a message', skipped: 'No message drafted' }
type Fields = { draft: string; subject: string; to: string; cc: string }
const addresses = (value: string) => value.split(/[,;\n]/).map((address) => address.trim()).filter(Boolean)
const messageOf = (fields: Fields): DraftMessage => ({ draft: fields.draft.trim(), subject: fields.subject.trim(), to: addresses(fields.to), cc: addresses(fields.cc) })

export function Draft({ record, channel, sender, error, drafting }: { record: CrmRecord; channel: string; sender?: DraftSender; error?: string; drafting?: Interaction['drafting'] }) {
  const current: Fields = { draft: text(record, 'draft'), subject: sender?.subject ?? text(record, 'subject'), to: (sender?.to ?? list(record, 'to')).join(', '), cc: (sender?.cc ?? list(record, 'cc')).join(', ') }
  const [edited, setEdited] = useState<Fields | null>(null)
  const fields = edited ?? current
  const message = messageOf(fields)
  const saving = useRef<{ key: string; promise: Promise<unknown> } | null>(null)
  const email = channel === 'Email'
  const write = useMutation({
    mutationKey: ['save_draft'],
    scope: { id: `draft:${record.id}` },
    mutationFn: ({ draft, subject, to, cc }: DraftMessage) => call('save_draft', { record_id: record.id, channel, draft, subject, to, cc }),
  })
  const commit = (next = message): Promise<unknown> => {
    const key = JSON.stringify(next)
    if (saving.current?.key === key) {
      return saving.current.promise
    }
    if (text(record, 'draft_status') === 'Sending' || (!saving.current && key === JSON.stringify(messageOf(current)))) {
      return Promise.resolve()
    }
    const promise = write.mutateAsync(next).then(() => {
      setEdited((latest) => latest && JSON.stringify(messageOf(latest)) === key ? null : latest)
    }).finally(() => {
      if (saving.current?.promise === promise) {
        saving.current = null
      }
    })
    saving.current = { key, promise }
    return promise
  }
  const person = (valuesOf(record, 'person')[0] as Ref | undefined)?.name
  const status = drafting && (drafting.state !== 'completed' || current.draft) ? drafting : undefined
  const idle = !(fields.draft || error || status) && 'hidden group-focus-within:flex'
  return (
    <div className="group rounded-[var(--radius-card)] bg-list-hover transition-colors duration-150 focus-within:bg-list-active" onKeyDown={(event) => event.stopPropagation()}>
      {email && <div className={cn('flex-col border-b border-border px-3.5 py-2 text-[12.5px] text-ink-3', idle)}>
        <div className="mb-1 flex items-center justify-between gap-3">
          <span>{sender?.reply ? 'Reply' : 'New email'}</span>
          {sender && <span className="truncate" title={sender.from}>From {sender.from}</span>}
        </div>
        {error && <p role="alert" className="mb-1 text-danger">{error}</p>}
        {(['to', 'cc', 'subject'] as const).map((field) => (
          <label key={field} className="flex min-w-0 items-center gap-2">
            <span className="w-12 shrink-0">{{ to: 'To', cc: 'Cc', subject: 'Subject' }[field]}</span>
            <input
              aria-label={{ to: 'To', cc: 'Cc', subject: 'Subject' }[field]}
              type="text"
              value={fields[field]}
              placeholder={field === 'subject' ? 'Add a subject' : field === 'to' ? 'Email address' : 'Optional'}
              onChange={(event) => setEdited({ ...fields, [field]: event.target.value })}
              onBlur={() => void commit().catch(() => {})}
              className="min-w-0 flex-1 bg-transparent py-1 text-ink-2 outline-none placeholder:text-ink-3 focus:text-ink"
            />
          </label>
        ))}
      </div>}
      <textarea
        aria-label="Draft"
        rows={1}
        placeholder={person ? `Message ${person}…` : 'Write a message…'}
        value={fields.draft}
        onChange={(event) => setEdited({ ...fields, draft: event.target.value })}
        onBlur={() => void commit().catch(() => {})}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            setEdited(null)
          } else if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
            event.preventDefault()
            event.currentTarget.blur()
          }
        }}
        className="field-sizing-content block max-h-[40dvh] min-h-10 w-full resize-none bg-transparent px-3.5 py-2.5 text-[13.5px] leading-[1.55] text-ink-2 outline-none placeholder:text-ink-3 focus:text-ink"
      />
      <div className={cn('flex items-center gap-2 py-2 pl-3.5 pr-2', idle)}>
        <p role="status" aria-live="polite" className="flex min-w-0 flex-1 items-start gap-1.5 text-[12px] leading-[18px] text-ink-3">
          {status?.state === 'drafting' && <LoaderCircle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 animate-spin motion-reduce:animate-none" />}
          {status ? <span>
            <span className={cn(status.state === 'failed' ? 'text-danger' : 'text-ink-2')}>{drafted[status.state]}</span>
            {status.reason && <> · {status.reason}</>}
          </span> : write.isPending ? 'Saving…' : current.draft && edited === null && 'Draft saved'}
        </p>
        <Release record={record} draft={message.draft} channel={channel} sender={sender && { ...sender, ...message }} beforeSend={commit} disabled={!!error} />
      </div>
    </div>
  )
}
