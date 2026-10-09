import { ChevronDown, X } from 'lucide-react'
import { recordName } from '@/lib/crm'
import { useDebounced } from '@/lib/hooks'
import { useTool, useWorkspace } from '@/lib/queries'
import { needsValue } from '@/lib/record-search'
import { filterOperators, type Attribute, type CrmObject, type CrmRecord, type RecordFilter } from '@/lib/types'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { inputClass } from './controls'
import { Picker } from './picker'

export const operatorNames: Record<RecordFilter['operator'], string> = {
  is: 'is', is_not: 'is not', contains: 'contains', not_contains: 'does not contain', is_empty: 'is empty', is_not_empty: 'is not empty',
  before: 'is before', on_or_before: 'is on or before', after: 'is after', on_or_after: 'is on or after',
}


export function FilterCondition({ object, filter, onChange, onRemove }: { object: CrmObject; filter: RecordFilter; onChange: (filter: RecordFilter) => void; onRemove: () => void }) {
  const attribute = object.attributes.find((a) => a.slug === filter.attribute)
  const operators = filterOperators.filter((op) => {
    if (['before', 'on_or_before', 'after', 'on_or_after'].includes(op)) {
      return ['date', 'datetime'].includes(attribute?.type ?? '')
    }
    return !['contains', 'not_contains'].includes(op) || !['reference', 'number', 'date', 'datetime', 'checkbox'].includes(attribute?.type ?? '')
  })
  return (
    <div className="flex flex-wrap items-center gap-1.5 rounded-[var(--radius-control)] bg-list-hover p-1.5">
      <Picker
        trigger={<Button variant="ghost" className="max-w-36"><span className="truncate">{attribute?.name ?? filter.attribute}</span><ChevronDown /></Button>}
        placeholder="Choose a field…"
        options={object.attributes.map((a) => ({ value: a.slug, label: a.name }))}
        selected={[filter.attribute]}
        onSelect={(value) => onChange({ attribute: value, operator: 'is' })}
      />
      <Picker
        trigger={<Button variant="ghost">{operatorNames[filter.operator]}<ChevronDown /></Button>}
        placeholder="Choose a condition…"
        options={operators.map((op) => ({ value: op, label: operatorNames[op] }))}
        selected={[filter.operator]}
        onSelect={(value) => {
          const operator = value as RecordFilter['operator']
          onChange({ attribute: filter.attribute, operator, ...(needsValue({ ...filter, operator }) ? { value: filter.value } : {}) })
        }}
      />
      {attribute && needsValue(filter) && <FilterValue key={filter.attribute} attribute={attribute} filter={filter} onChange={(value) => onChange({ ...filter, value })} />}
      <Button variant="ghost" size="icon" className="ml-auto" aria-label={`Remove ${attribute?.name ?? filter.attribute} condition`} onClick={onRemove}><X /></Button>
    </div>
  )
}

function FilterValue({ attribute, filter, onChange }: { attribute: Attribute; filter: RecordFilter; onChange: (value: string) => void }) {
  const [search, setSearch] = useState('')
  const query = useDebounced(search)
  const reference = attribute.type === 'reference'
  const records = useTool<{ records: CrmRecord[] }>('search_records', { object: attribute.target, query, limit: 100 }, { enabled: reference }).data?.records ?? []
  const selected = useTool<CrmRecord>('get_record', { record_id: filter.value }, { enabled: reference && !!filter.value }).data
  const member = attribute.type === 'member'
  const members = useWorkspace()?.members ?? []
  const choice = ['select', 'status', 'checkbox'].includes(attribute.type) || reference || member
  if (attribute.type === 'date' || attribute.type === 'datetime') {
    return (
      <div className="flex min-w-0 flex-1 items-center gap-1">
        <Button variant="ghost" aria-pressed={filter.value === 'today'} onClick={() => onChange(filter.value === 'today' ? '' : 'today')} className={filter.value === 'today' ? 'bg-list-active' : undefined}>Today</Button>
        {attribute.type === 'datetime' && <Button variant="ghost" aria-pressed={filter.value === 'now'} onClick={() => onChange(filter.value === 'now' ? '' : 'now')} className={filter.value === 'now' ? 'bg-list-active' : undefined}>Now</Button>}
        {!['today', 'now'].includes(filter.value ?? '') && <input aria-label={`${attribute.name} value`} value={filter.value ?? ''} onChange={(e) => onChange(e.target.value)} type="date" className={`${inputClass} min-w-0 flex-1`} />}
      </div>
    )
  }
  if (!choice || ['contains', 'not_contains'].includes(filter.operator)) {
    return <input autoFocus aria-label={`${attribute.name} value`} value={filter.value ?? ''} onChange={(e) => onChange(e.target.value)} type={attribute.type === 'number' ? 'number' : 'text'} placeholder="Value…" className={`${inputClass} min-w-28 flex-1`} />
  }
  const options = reference
    ? records.map((r) => ({ value: r.id, label: recordName(r) }))
    : member
      ? members.map((m) => ({ value: m.email, label: m.name || m.email }))
      : attribute.type === 'checkbox'
        ? [{ value: 'true', label: 'Yes' }, { value: 'false', label: 'No' }]
        : (attribute.options ?? []).map((value) => ({ value, label: value }))
  return (
    <Picker
      trigger={<Button variant="ghost" className="min-w-28 flex-1 justify-between"><span className="max-w-48 truncate">{(selected ? recordName(selected) : options.find((o) => o.value === filter.value)?.label) ?? filter.value ?? 'Choose value…'}</span><ChevronDown /></Button>}
      placeholder={`Find ${attribute.name.toLowerCase()}…`}
      options={options}
      selected={filter.value ? [filter.value] : []}
      onSelect={onChange}
      onSearch={reference ? setSearch : undefined}
    />
  )
}
