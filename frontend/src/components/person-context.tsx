import { useState } from 'react'
import { contextHint, valueText, valuesOf } from '@/lib/crm'
import { useWrite } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'

export function PersonContext({ record }: { record: CrmRecord }) {
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
    <section aria-label="Context" className="mt-6">
      <h2 className="mb-1.5 text-[12px] font-medium text-ink-3">Context</h2>
      <textarea
        aria-label="Context"
        value={text}
        disabled={write.pending}
        rows={3}
        placeholder={contextHint}
        onChange={(e) => setText(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            setText(current)
          } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            e.preventDefault()
            e.currentTarget.blur()
          }
        }}
        className="field-sizing-content block max-h-80 min-h-20 w-full resize-y rounded-[var(--radius-control)] border border-transparent bg-transparent px-2 py-2 text-[13px] leading-5 text-ink outline-none placeholder:text-ink-3 hover:bg-list-hover focus:border-border focus:bg-bg"
      />
    </section>
  )
}
