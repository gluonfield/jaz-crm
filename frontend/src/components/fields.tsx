import { Link } from '@tanstack/react-router'
import { Check, Plus } from 'lucide-react'
import { useState } from 'react'
import { recordName, valueKey, valueText, valuesOf } from '@/lib/crm'
import { formatDay, formatNumber } from '@/lib/format'
import { useDebounced } from '@/lib/hooks'
import { useRecords, useWorkspace, useWrite } from '@/lib/queries'
import type { Attribute, CrmRecord, Value } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Chip } from './controls'
import { RecordIcon } from './icons'
import { Picker } from './picker'
import { Stage, StageDot } from './stage'
import { SelectField } from './select-field'

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
  if (attribute.type === 'select') {
    return <SelectField record={record} attribute={attribute} />
  }
  if (attribute.type === 'status') {
    const current = values[0] && valueText(values[0])
    return (
      <Picker
        trigger={
          <button className={cn(valueButton, !current && 'text-ink-3')}>{current ? <Stage stage={current} /> : 'Set…'}</button>
        }
        placeholder={`Set ${attribute.name.toLowerCase()}...`}
        options={(attribute.options ?? []).map((o) => ({ value: o, label: o, icon: <StageDot stage={o} /> }))}
        selected={values.map(valueKey)}
        onSelect={(option) => write.set(slug, option)}
      />
    )
  }
  if (attribute.type === 'member') {
    return <MemberField attribute={attribute} values={values.map(valueText)} onSelect={(email) => write.set(slug, email)} />
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
      display={current ? (attribute.type === 'number' ? formatNumber(current, slug) : attribute.type === 'date' ? formatDay(current) : undefined) : undefined}
      onCommit={(text) => (text ? write.set(slug, text) : current && write.remove(slug, []))}
    />
  )
}

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
  // An empty date shows its placeholder rather than the browser's date mask.
  const shown = display ?? (type === 'date' && !text ? placeholder : undefined)
  if (shown && !editing) {
    return (
      <button type="button" aria-label={display ? `${label}: ${display}` : label} className={cn(valueButton, !display && 'text-ink-3', className)} onClick={() => setEditing(true)}>
        {shown}
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

// MemberField picks a member of the workspace, shown by name and photo.
function MemberField({ attribute, values, onSelect }: { attribute: Attribute; values: string[]; onSelect: (email: string) => void }) {
  const members = useWorkspace()?.members ?? []
  const chosen = members.find((m) => m.email === values[0])
  return (
    <Picker
      trigger={
        <button className={cn(valueButton, 'gap-2', !values[0] && 'text-ink-3')}>
          {chosen && <RecordIcon object="people" name={chosen.name || chosen.email} photo={chosen.photo} size={18} />}
          <span className="truncate">{chosen ? chosen.name || chosen.email : (values[0] ?? 'Set…')}</span>
        </button>
      }
      placeholder={`Set ${attribute.name.toLowerCase()}…`}
      options={members.map((m) => ({ value: m.email, label: m.name || m.email, icon: <RecordIcon object="people" name={m.name || m.email} photo={m.photo} size={18} /> }))}
      selected={values}
      onSelect={onSelect}
    />
  )
}
