import { ChevronDown, Tags } from 'lucide-react'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { useAction, useWrite } from '@/lib/queries'
import type { Attribute, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Picker } from './picker'

const colours = [
  'bg-amber-500/12 text-amber-700 ring-amber-500/20 dark:text-amber-300',
  'bg-violet-500/12 text-violet-700 ring-violet-500/20 dark:text-violet-300',
  'bg-emerald-500/12 text-emerald-700 ring-emerald-500/20 dark:text-emerald-300',
  'bg-rose-500/12 text-rose-700 ring-rose-500/20 dark:text-rose-300',
  'bg-sky-500/12 text-sky-700 ring-sky-500/20 dark:text-sky-300',
  'bg-orange-500/12 text-orange-700 ring-orange-500/20 dark:text-orange-300',
]

const colourOf = (value: string) => colours[[...value.toLowerCase()].reduce((hash, letter) => (hash * 31 + letter.codePointAt(0)!) % colours.length, 0)]

export function ValueTag({ value }: { value: string }) {
  return (
    <span title={value} className={cn('inline-flex h-6 max-w-48 items-center rounded-full px-2.5 text-[12px] font-medium ring-1 ring-inset', colourOf(value))}>
      <span className="truncate">{value}</span>
    </span>
  )
}

// ValueDot is a value's colour alone, as menus show it.
export function ValueDot({ value }: { value: string }) {
  return <span className={cn(colourOf(value), 'size-2 shrink-0 rounded-full bg-current')} />
}

export function SelectField({ record, attribute }: { record: CrmRecord; attribute: Attribute }) {
  const values = valuesOf(record, attribute.slug).map(valueText)
  const write = useWrite(record)
  const add = useAction<{ object: string; attribute: string; value: string }, { value: string }>('add_attribute_option')
  const pending = write.pending || add.isPending
  return (
    <Picker
      trigger={
        <button
          type="button"
          aria-label={`Edit ${attribute.name.toLowerCase()} for ${recordName(record)}`}
          disabled={pending}
          onKeyDown={(e) => e.stopPropagation()}
          className="flex min-h-7 w-full min-w-0 items-center gap-1.5 rounded-[var(--radius-control)] px-1.5 py-0.5 text-left outline-none hover:bg-list-hover focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
        >
          <span className="flex min-w-0 flex-1 flex-wrap gap-1">
            {values.length ? values.map((value) => <ValueTag key={value} value={value} />) : <span className="flex items-center gap-1.5 text-[12px] text-ink-3"><Tags className="size-3.5" /> Add {attribute.name.toLowerCase()}</span>}
          </span>
          <ChevronDown className="size-3 shrink-0 text-ink-3" />
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
