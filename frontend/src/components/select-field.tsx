import { ChevronDown, Tags } from 'lucide-react'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { useAction, useWrite } from '@/lib/queries'
import type { Attribute, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Picker } from './picker'

// A value is a dot of colour and its word: colour sorts values at a glance
// without filling a table with pills.
const colours = ['text-amber-500', 'text-violet-400', 'text-emerald-500', 'text-rose-400', 'text-sky-400', 'text-orange-400']

const colourOf = (value: string) => colours[[...value.toLowerCase()].reduce((hash, letter) => (hash * 31 + letter.codePointAt(0)!) % colours.length, 0)]

export function ValueTag({ value }: { value: string }) {
  return (
    <span title={value} className={cn('inline-flex min-h-6 min-w-0 max-w-48 items-center gap-2 text-[13px] before:size-2 before:shrink-0 before:rounded-full before:bg-current', colourOf(value))}>
      <span className="truncate text-ink">{value}</span>
    </span>
  )
}

// ValueDot is a value's colour alone, as menus show it.
export function ValueDot({ value }: { value: string }) {
  return <span className={cn(colourOf(value), 'size-2 shrink-0 rounded-full bg-current')} />
}

// SelectField picks a select's values; inline, as in a table, it keeps to one
// line and shows its empty label and menu chevron only on hover.
export function SelectField({ record, attribute, inline = false, onCommit }: { record: CrmRecord; attribute: Attribute; inline?: boolean; onCommit?: () => void }) {
  const values = valuesOf(record, attribute.slug).map(valueText)
  const limit = inline ? 1 : Infinity
  const write = useWrite(record, onCommit)
  const add = useAction<{ object: string; attribute: string; value: string }, { value: string }>('add_attribute_option')
  const pending = write.pending || add.isPending
  return (
    <Picker
      trigger={
        <button
          type="button"
          aria-label={`Edit ${attribute.name.toLowerCase()} for ${recordName(record)}`}
          disabled={pending}
          className="flex min-h-7 w-full min-w-0 items-center gap-1.5 rounded-[var(--radius-control)] px-1.5 py-0.5 text-left outline-none hover:bg-list-hover focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
        >
          <span className={cn('flex min-w-0 flex-1 gap-x-3 gap-y-1', inline ? 'overflow-hidden' : 'flex-wrap')}>
            {values.length ? values.slice(0, limit).map((value) => <ValueTag key={value} value={value} />) : <span data-hint className="flex items-center gap-1.5 text-[12px] text-ink-3"><Tags className="size-3.5" /> Add {attribute.name.toLowerCase()}</span>}
            {values.length > limit && <span className="shrink-0 self-center text-[12px] text-ink-3">+{values.length - limit}</span>}
          </span>
          <ChevronDown data-hint={inline || undefined} className="size-3 shrink-0 text-ink-3" />
        </button>
      }
      placeholder={`Find or create ${attribute.name.toLowerCase()}…`}
      options={(attribute.options ?? []).map((value) => ({ value, label: value }))}
      selected={values}
      multiple={attribute.multi}
      disabled={pending}
      onSelect={(value) => values.includes(value) ? write.remove(attribute.slug, [value]) : write.set(attribute.slug, value)}
      onCreate={(value) => add.mutate({ object: record.object, attribute: attribute.slug, value }, { onSuccess: (out) => write.set(attribute.slug, out.value) })}
    />
  )
}
