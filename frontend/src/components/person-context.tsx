import { useState } from 'react'
import { contextHint, valueText, valuesOf } from '@/lib/crm'
import { useWrite } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'

export function PersonContext({ record }: { record: CrmRecord }) {
  return (
    <section aria-label="Context" className="mt-6">
      <h2 className="mb-1.5 text-[12px] font-medium text-ink-3">Context</h2>
      <ContextEditor record={record} className="min-h-20 resize-y border border-transparent px-2 py-2 hover:bg-list-hover focus:border-border focus:bg-bg" />
    </section>
  )
}

// ContextEditor edits what we know of a person, saving on blur; Escape puts
// back what was saved.
export function ContextEditor({ record, autoFocus, onDone, className }: { record: CrmRecord; autoFocus?: boolean; onDone?: () => void; className?: string }) {
  const current = valuesOf(record, 'context').map(valueText).join('\n')
  const [text, setText] = useState(current)
  const write = useWrite(record)
  const commit = () => {
    const next = text.trim()
    if (next !== current && !write.pending) {
      if (next) {
        write.set('context', next)
      } else {
        write.remove('context', [])
      }
    }
  }
  return (
    <textarea
      aria-label="Context"
      autoFocus={autoFocus}
      value={text}
      disabled={write.pending}
      rows={3}
      placeholder={contextHint}
      onChange={(e) => setText(e.target.value)}
      onBlur={() => {
        commit()
        onDone?.()
      }}
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          setText(current)
          onDone?.()
        } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
          e.preventDefault()
          e.currentTarget.blur()
        }
      }}
      className={cn('field-sizing-content block max-h-80 w-full rounded-[var(--radius-control)] bg-transparent text-[13px] leading-5 text-ink outline-none placeholder:text-ink-3', className)}
    />
  )
}
