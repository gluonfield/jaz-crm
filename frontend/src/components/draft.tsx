import { useLayoutEffect, useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { LoaderCircle } from 'lucide-react'
import { call } from '@/lib/api'
import { valueText, valuesOf } from '@/lib/crm'
import type { CrmRecord, DraftMessage, DraftProposal, DraftRewriteAction, DraftSender, Interaction, Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Release } from './draft-release'
import { DraftRewrite } from './draft-rewrite'

const list = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText)
const text = (record: CrmRecord, slug: string) => list(record, slug).join(', ')
const drafted = { drafting: 'Drafting a message…', completed: 'Message drafted', failed: 'Couldn’t draft a message', skipped: 'No message drafted' }
type Fields = { draft: string; subject: string; to: string; cc: string; revision?: string }
const addresses = (value: string) => value.split(/[,;\n]/).map((address) => address.trim()).filter(Boolean)
const messageOf = (fields: Fields): DraftMessage => ({ draft: fields.draft.trim(), subject: fields.subject.trim(), to: addresses(fields.to), cc: addresses(fields.cc), revision: fields.revision })
const sameContent = (a: DraftProposal, b: DraftProposal) => a.draft === b.draft && a.subject === b.subject
const sameFields = (a: Fields, b: Fields) => sameContent(a, b) && a.to === b.to && a.cc === b.cc

export function Draft({ record, channel, sender, error, drafting }: { record: CrmRecord; channel: string; sender?: DraftSender; error?: string; drafting?: Interaction['drafting'] }) {
  const current: Fields = { draft: text(record, 'draft'), subject: sender?.subject ?? text(record, 'subject'), to: (sender?.to ?? list(record, 'to')).join(', '), cc: (sender?.cc ?? list(record, 'cc')).join(', '), revision: sender?.revision }
  const [edited, setEdited] = useState<Fields | null>(null)
  if (edited && JSON.stringify(messageOf(edited)) === JSON.stringify(messageOf(current))) {
    setEdited(null)
  }
  const fields = edited ?? current
  const message = messageOf(fields)
  const sending = text(record, 'draft_status') === 'Sending'
  const editor = useRef({ fields, current, sending })
  useLayoutEffect(() => {
    editor.current = { fields, current, sending }
  })
  const saving = useRef<{ key: string; savedKey?: string; revision?: string; promise: Promise<string | undefined>; pending: boolean } | null>(null)
  const rewriting = useRef<object | null>(null)
  useLayoutEffect(() => () => {
    rewriting.current = null
  }, [])
  const [rewritePending, setRewritePending] = useState(false)
  const [rewriteError, setRewriteError] = useState('')
  const [undo, setUndo] = useState<{ before: DraftProposal; after: DraftProposal } | null>(null)
  const email = channel === 'Email'
  const write = useMutation({
    mutationKey: ['save_draft'],
    scope: { id: `draft:${record.id}` },
    mutationFn: (message: DraftMessage) => call<{ revision?: string }>('save_draft', { record_id: record.id, channel, ...message }),
  })
  const commit = (next = message): Promise<string | undefined> => {
    const key = JSON.stringify(next)
    const persisted = JSON.stringify(messageOf(editor.current.current))
    if ((saving.current?.key === key || saving.current?.savedKey === key) && (saving.current.pending || saving.current.savedKey === persisted)) {
      return saving.current.promise
    }
    if (editor.current.sending || (!saving.current?.pending && key === persisted)) {
      return Promise.resolve(next.revision)
    }
    const previous = saving.current
    const revision = next.revision && previous?.revision === next.revision ? previous.promise.catch(() => next.revision) : Promise.resolve(next.revision)
    const promise = revision.then((revision) => write.mutateAsync({ ...next, revision })).then(({ revision }) => {
      if (saving.current?.promise === promise) {
        saving.current.savedKey = JSON.stringify({ ...next, revision })
        setEdited((latest) => latest && latest.revision === next.revision ? { ...latest, revision } : latest)
      }
      return revision
    }).catch((error: unknown) => {
      if (saving.current?.promise === promise) {
        saving.current = null
      }
      throw error
    }).finally(() => {
      if (saving.current?.promise === promise) {
        saving.current.pending = false
      }
    })
    saving.current = { key, revision: next.revision, promise, pending: true }
    return promise
  }
  const rewrite = async (action: DraftRewriteAction, instruction?: string) => {
    if (rewriting.current || editor.current.sending) {
      return
    }
    const request = {}
    rewriting.current = request
    setRewritePending(true)
    setRewriteError('')
    const { fields: before, current: persisted } = editor.current
    const { revision: _revision, ...input } = messageOf(before)
    try {
      const proposal = await call<DraftProposal>('rewrite_draft', { record_id: record.id, ...input, from: sender?.from, action, instruction })
      if (rewriting.current !== request) {
        return
      }
      if (editor.current.sending || !sameFields(editor.current.fields, before) || (!sameFields(editor.current.current, persisted) && !sameFields(editor.current.current, before))) {
        setRewriteError('Your draft changed while AI was editing. Try again with the current text.')
        return
      }
      const next = { ...editor.current.fields, ...proposal }
      setEdited(next)
      setUndo({ before, after: proposal })
      await commit(messageOf(next))
    } catch (error) {
      if (rewriting.current === request) {
        setRewriteError(error instanceof Error ? error.message : 'Could not rewrite the draft. Try again.')
      }
    } finally {
      if (rewriting.current === request) {
        rewriting.current = null
        setRewritePending(false)
      }
    }
  }
  const undoRewrite = () => {
    if (!undo || rewriting.current || editor.current.sending || !sameContent(editor.current.fields, undo.after)) {
      return
    }
    const next = { ...editor.current.fields, draft: undo.before.draft, subject: undo.before.subject }
    setEdited(next)
    setUndo(null)
    setRewriteError('')
    void commit(messageOf(next)).catch(() => {})
  }
  const person = (valuesOf(record, 'person')[0] as Ref | undefined)?.name
  const status = drafting && (drafting.state !== 'completed' || current.draft) ? drafting : undefined
  const idle = !(fields.draft || error || status) && 'hidden group-focus-within:flex'
  return (
    <div className="group rounded-[var(--radius-card)] bg-list-hover transition-colors duration-150 focus-within:bg-list-active" onKeyDown={(event) => event.stopPropagation()}>
      {email && <div className={cn('flex-col border-b border-border px-3.5 py-2 text-[12.5px] text-ink-3', idle)}>
        <div className="mb-1 flex items-center justify-between gap-3">
          <span>{sender?.imported_draft ? 'Gmail draft' : sender?.reply ? 'Reply' : 'New email'}</span>
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
        {!!sender?.bcc?.length && <p className="flex gap-2 py-1"><span className="w-12 shrink-0">Bcc</span><span className="min-w-0 text-ink-2 [overflow-wrap:anywhere]">{sender.bcc.join(', ')}</span></p>}
        {!!sender?.attachments?.length && <p className="py-1 text-ink-2 [overflow-wrap:anywhere]">Attachments: {sender.attachments.join(', ')}</p>}
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
      {(message.draft || rewritePending) && <DraftRewrite pending={rewritePending} disabled={sending} canUndo={!!undo && sameContent(fields, undo.after)} onRewrite={(action, instruction) => void rewrite(action, instruction)} onUndo={undoRewrite} />}
      {(rewriteError || write.error) && <p role="alert" className="px-3.5 pb-1 text-[12px] text-danger">{rewriteError || write.error?.message}</p>}
      <div className={cn('flex items-center gap-2 py-2 pl-3.5 pr-2', idle)}>
        <p role="status" aria-live="polite" className="flex min-w-0 flex-1 items-start gap-1.5 text-[12px] leading-[18px] text-ink-3">
          {status?.state === 'drafting' && <LoaderCircle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 animate-spin motion-reduce:animate-none" />}
          {status ? <span>
            <span className={cn(status.state === 'failed' ? 'text-danger' : 'text-ink-2')}>{drafted[status.state]}</span>
            {status.reason && <> · {status.reason}</>}
          </span> : write.isPending ? 'Saving…' : current.draft && edited === null && 'Draft saved'}
        </p>
        <Release record={record} draft={message.draft} channel={channel} sender={sender && { ...sender, ...message, html: sender.draft?.trim() === message.draft && sender.revision === message.revision ? sender.html : undefined }} beforeSend={commit} disabled={!!error || rewritePending} />
      </div>
    </div>
  )
}
