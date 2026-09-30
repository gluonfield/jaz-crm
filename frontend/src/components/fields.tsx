import { Link } from '@tanstack/react-router'
import { Check, Plus } from 'lucide-react'
import { useState } from 'react'
import { recordName, valueKey, valueText, valuesOf } from '@/lib/crm'
import { formatDay } from '@/lib/format'
import { useDebounced } from '@/lib/hooks'
import { useAction, useRecords } from '@/lib/queries'
import type { Attribute, CrmRecord, Value } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Chip } from './controls'
import { RecordIcon } from './icons'
import { Picker } from './picker'
import { Stage, StageDot } from './stage'

type Write = { object: string; record_id: string; values?: Record<string, string | string[]>; remove?: Record<string, string[]> }

// useWrite sets or removes one attribute's values on a record.
export function useWrite(record: CrmRecord) {
  const upsert = useAction<Write>('upsert_record')
  return {
    set: (slug: string, value: string) => upsert.mutate({ object: record.object, record_id: record.id, values: { [slug]: value } }),
    remove: (slug: string, values: string[]) => upsert.mutate({ object: record.object, record_id: record.id, remove: { [slug]: values } }),
  }
}

const inputType: Partial<Record<Attribute['type'], string>> = { number: 'number', date: 'date', email: 'email', url: 'url', phone: 'tel' }

// Field shows and edits one attribute of a record in the way its type needs.
export function Field({ record, attribute }: { record: CrmRecord; attribute: Attribute }) {
  const values = valuesOf(record, attribute.slug)
  const write = useWrite(record)
  const slug = attribute.slug
  if (attribute.type === 'checkbox') {
    const on = values[0] === 'true'
    return (
      <button
        type="button"
        aria-label={attribute.name}
        aria-pressed={on}
        onClick={() => write.set(slug, String(!on))}
        className={cn('flex size-4 items-center justify-center rounded-[4px] border border-ink-3/60', on && 'border-primary bg-primary text-on-primary')}
      >
        {on && <Check className="size-3" strokeWidth={3} />}
      </button>
    )
  }
  if (attribute.type === 'select' || attribute.type === 'status') {
    const status = attribute.type === 'status'
    const current = values[0] && valueText(values[0])
    return (
      <Picker
        trigger={
          <button className={cn(valueButton, !current && 'text-ink-3')}>{current && status ? <Stage attribute={attribute} stage={current} /> : (current ?? 'Set…')}</button>
        }
        placeholder={`Set ${attribute.name.toLowerCase()}...`}
        options={(attribute.options ?? []).map((o) => ({ value: o, label: o, icon: status ? <StageDot attribute={attribute} stage={o} /> : undefined }))}
        selected={values.map(valueKey)}
        onSelect={(option) => (option === current && !status ? write.remove(slug, [option]) : write.set(slug, option))}
      />
    )
  }
  if (attribute.type === 'reference') {
    return <ReferenceField record={record} attribute={attribute} values={values} />
  }
  if (attribute.multi) {
    return (
      <div className="flex min-w-0 flex-wrap items-center gap-1">
        {values.map((v) => (
          <Chip key={valueKey(v)} onRemove={() => write.remove(slug, [valueKey(v)])}>
            {valueText(v)}
          </Chip>
        ))}
        <TextInput
          key={values.length}
          label={`Add ${attribute.name.toLowerCase()}`}
          type={inputType[attribute.type]}
          placeholder="Add…"
          className="w-auto min-w-16 flex-1"
          onCommit={(text) => write.set(slug, text)}
        />
      </div>
    )
  }
  const current = values[0] ? valueText(values[0]) : ''
  return (
    <TextInput
      key={current}
      initial={current}
      label={attribute.name}
      type={inputType[attribute.type]}
      placeholder="Add…"
      display={current ? shown[attribute.type]?.(current) : undefined}
      onCommit={(text) => (text ? write.set(slug, text) : current && write.remove(slug, []))}
    />
  )
}

const number = new Intl.NumberFormat('en', { maximumFractionDigits: 2 })

// shown formats the values whose stored form reads poorly.
const shown: Partial<Record<Attribute['type'], (value: string) => string>> = { date: formatDay, number: (value) => number.format(Number(value)) }

const valueButton = 'flex h-7 min-w-0 max-w-full items-center truncate rounded-[var(--radius-control)] px-1.5 text-left text-[13px] text-ink outline-none hover:bg-list-hover'

// TextInput edits in place and commits on Enter or blur; a required one
// emptied goes back to what it was.
export function TextInput({
  initial = '',
  label,
  type,
  placeholder,
  display,
  required,
  className,
  onCommit,
}: {
  initial?: string
  label: string
  type?: string
  placeholder: string
  display?: string
  required?: boolean
  className?: string
  onCommit: (text: string) => void
}) {
  const [text, setText] = useState(initial)
  const [editing, setEditing] = useState(false)
  const commit = () => {
    setEditing(false)
    if (required && !text.trim()) {
      setText(initial)
    } else if (text.trim() !== initial) {
      onCommit(text.trim())
    }
  }
  if (display && !editing) {
    return (
      <button type="button" aria-label={`${label}: ${display}`} className={cn(valueButton, className)} onClick={() => setEditing(true)}>
        {display}
      </button>
    )
  }
  return (
    <input
      value={text}
      aria-label={label}
      type={type}
      placeholder={placeholder}
      autoFocus={editing}
      onChange={(e) => setText(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === 'Enter') {
          e.currentTarget.blur()
        } else if (e.key === 'Escape') {
          setText(initial)
          setEditing(false)
        }
      }}
      className={cn(
        'h-7 w-full min-w-0 rounded-[var(--radius-control)] border border-transparent bg-transparent px-1.5 text-[13px] text-ink outline-none placeholder:text-ink-3 hover:bg-list-hover focus:border-border focus:bg-bg',
        className,
      )}
    />
  )
}

function ReferenceField({ record, attribute, values }: { record: CrmRecord; attribute: Attribute; values: Value[] }) {
  const write = useWrite(record)
  const [search, setSearch] = useState('')
  const found = useRecords(attribute.target ?? '', useDebounced(search), 20).data?.records ?? []
  const target = attribute.target ?? ''
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-1">
      {values.map((v) => (
        <Chip key={valueKey(v)} onRemove={() => write.remove(attribute.slug, [valueKey(v)])}>
          <Link to="/r/$recordId" params={{ recordId: valueKey(v) }} className="flex items-center gap-1.5 hover:text-ink">
            <RecordIcon object={target} name={valueText(v)} size={14} />
            {valueText(v)}
          </Link>
        </Chip>
      ))}
      {(attribute.multi || values.length === 0) && (
        <Picker
          trigger={
            <button className="flex size-6 items-center justify-center rounded-[var(--radius-control)] text-ink-3 outline-none hover:bg-list-hover hover:text-ink" aria-label={`Set ${attribute.name}`}>
              <Plus className="size-3.5" />
            </button>
          }
          placeholder={`Find ${target}...`}
          onSearch={setSearch}
          options={found.map((r) => ({ value: r.id, label: recordName(r), icon: <RecordIcon object={target} name={recordName(r)} size={16} /> }))}
          onSelect={(id) => write.set(attribute.slug, id)}
        />
      )}
    </div>
  )
}

