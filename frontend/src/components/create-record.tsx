import { DateField, dateMetadata } from './date-field'
import { Button } from '@jaz/ui/button'
import { useNavigate } from '@tanstack/react-router'
import { X } from 'lucide-react'
import { type ReactNode, forwardRef, useState } from 'react'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { contextHint, recordName, valueKey, valuesOf } from '@/lib/crm'
import { useDebounced } from '@/lib/hooks'
import { useAction, useRecords, useWorkspace } from '@/lib/queries'
import type { Attribute, CrmObject, CrmRecord, Member } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Kbd } from './kbd'
import { ObjectIcon, RecordIcon } from './icons'
import { Picker } from './picker'
import { StageDot } from './stage'
import { ValueTag } from './select-field'

type Draft = Record<string, string[]>
type Label = { name: string; photo?: string }

// singular names one of an object's records: person, company, deal.
export function singular(object: CrmObject) {
  const name = object.name.toLowerCase()
  if (name === 'people') {
    return 'person'
  }
  return name.endsWith('ies') ? `${name.slice(0, -3)}y` : name.replace(/s$/, '')
}

const capitalized = (text: string) => text[0].toUpperCase() + text.slice(1)

// CreateRecord adds a record: its name, the value that identifies it, and a
// row of its other properties. A board opens it in a stage and stays; the
// New button opens the record once created. Given a record, it edits that
// record instead and writes only what changed.
export function CreateRecord({
  object,
  open,
  onOpenChange,
  initial,
  record,
  openCreated = false,
}: {
  object: CrmObject
  open: boolean
  onOpenChange: (open: boolean) => void
  initial?: Record<string, string>
  record?: CrmRecord
  openCreated?: boolean
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        onOpenAutoFocus={(e) => {
          const name = (e.currentTarget as HTMLElement).querySelector('input')
          e.preventDefault()
          name?.focus()
        }}
        className="top-[12%] w-[620px] max-w-[calc(100vw-2rem)] translate-y-0 gap-0 rounded-[12px] border-border bg-raised p-0 shadow-[var(--shadow-raised)] sm:max-w-[620px]"
      >
        {open && <Form object={object} initial={initial} record={record} openCreated={openCreated} close={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function Form({ object, initial = {}, record, openCreated, close }: { object: CrmObject; initial?: Record<string, string>; record?: CrmRecord; openCreated: boolean; close: () => void }) {
  const members = useWorkspace()?.members ?? []
  const me = members.find((m) => m.is_me)
  const identity = object.attributes.find((a) => a.unique)
  const properties = object.attributes.filter((a) => !(object.slug === 'follow_ups' && (dateMetadata(a.slug) || a.slug === 'action_date')) && a.slug !== 'name' && a !== identity && a.type !== 'checkbox' && a.type !== 'markdown')
  const held = (slug: string) => (record ? valuesOf(record, slug).map(valueKey) : [])
  const [name, setName] = useState(held('name')[0] ?? '')
  const [key, setKey] = useState(identity ? held(identity.slug).join(', ') : '')
  const [draft, setDraft] = useState<Draft>(() =>
    Object.fromEntries(
      properties.flatMap((a) => {
        if (record) {
          const values = held(a.slug)
          return values.length ? [[a.slug, typed(a) ? [values.join(', ')] : values]] : []
        }
        const value = initial[a.slug] ?? (a.type === 'status' ? a.options?.[0] : a.type === 'member' ? me?.email : undefined)
        return value ? [[a.slug, [value]]] : []
      }),
    ),
  )
  const [labels, setLabels] = useState<Record<string, Label>>(() =>
    Object.fromEntries(Object.values(record?.values ?? {}).flat().flatMap((v) => (v && typeof v !== 'string' ? [[v.id, { name: v.name ?? '', photo: v.photo }]] : []))),
  )
  const upsert = useAction<object, { record: CrmRecord }>('upsert_record')
  const navigate = useNavigate()
  const set = (slug: string, values: string[]) => setDraft((d) => ({ ...d, [slug]: values }))
  const ready = !!(name.trim() || key.trim()) && !upsert.isPending
  const submit = () => {
    if (!ready) {
      return
    }
    // Multi-valued attributes gain values and lose the ones taken out; others
    // are replaced, or cleared when emptied.
    const values: Record<string, string | string[]> = {}
    const remove: Record<string, string[]> = {}
    const write = (a: Attribute, entries: string[]) => {
      const next = entries.flatMap((v) => (a.multi && typed(a) ? v.split(',') : [v])).map((v) => v.trim()).filter(Boolean)
      const before = held(a.slug)
      if (a.multi) {
        const added = next.filter((v) => !before.includes(v))
        const gone = before.filter((v) => !next.includes(v))
        if (added.length) {
          values[a.slug] = added
        }
        if (gone.length) {
          remove[a.slug] = gone
        }
      } else if (next[0] && next[0] !== before[0]) {
        values[a.slug] = next[0]
      } else if (!next[0] && before[0]) {
        remove[a.slug] = []
      }
    }
    for (const a of object.attributes) {
      if (a.slug === 'name') {
        write(a, [name])
      } else if (a === identity) {
        write(a, [key])
      } else if (properties.includes(a)) {
        write(a, draft[a.slug] ?? [])
      }
    }
    if (record && !Object.keys(values).length && !Object.keys(remove).length) {
      close()
      return
    }
    upsert.mutate(
      { object: object.slug, record_id: record?.id, values, remove },
      {
        onSuccess: (out) => {
          close()
          if (openCreated) {
            void navigate({ to: '/r/$recordId', params: { recordId: out.record.id } })
          }
        },
      },
    )
  }
  return (
    <form
      className="min-w-0"
      onSubmit={(e) => {
        e.preventDefault()
        submit()
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
          e.preventDefault()
          submit()
        }
      }}
    >
      <div className="flex items-center gap-2 px-4 pt-3.5 text-[12.5px] text-ink-2">
        <ObjectIcon slug={object.slug} className="size-3.5" />
        <DialogTitle className="text-[12.5px] font-normal">{record ? 'Edit' : 'New'} {singular(object)}</DialogTitle>
        <Button aria-label="Close" onClick={close} variant="ghost" size="icon-sm" className="ml-auto">
          <X className="size-4" />
        </Button>
      </div>
      <div className="px-4 pt-3">
        <input
          value={name}
          aria-label="Name"
          onChange={(e) => setName(e.target.value)}
          placeholder={singular(object) === 'person' ? 'Full name' : `${capitalized(singular(object))} name`}
          className="w-full bg-transparent text-[18px] font-semibold text-ink outline-none placeholder:text-ink-3"
        />
        {identity && (
          <input
            value={key}
            aria-label={identity.name}
            onChange={(e) => setKey(e.target.value)}
            placeholder={identity.multi ? identity.name.replace(/es$|s$/, '') : identity.name}
            className="mt-1.5 w-full bg-transparent text-[14px] text-ink outline-none placeholder:text-ink-3"
          />
        )}
      </div>
      <div className="flex flex-wrap items-center gap-1.5 px-4 pb-4 pt-4">
        {record?.object === 'follow_ups' && <DateField record={record} />}
        {properties.map((a) => (
          <Property key={a.slug} attribute={a} values={draft[a.slug] ?? []} labels={labels} members={members} onChange={(values) => set(a.slug, values)} onLabel={(id, label) => setLabels((l) => ({ ...l, [id]: label }))} />
        ))}
      </div>
      <div className="flex items-center justify-end gap-3 border-t border-border px-4 py-2.5">
        <Button
          type="submit"
          disabled={!ready}
          variant="primary" size="lg"
        >
          {record ? 'Save' : `Create ${singular(object)}`}
          <span className="flex items-center gap-0.5 opacity-70">
            <Kbd className="ml-0 border-on-primary/25 bg-on-primary/15 text-on-primary">⌘</Kbd>
            <Kbd className="ml-0 border-on-primary/25 bg-on-primary/15 text-on-primary">↵</Kbd>
          </span>
        </Button>
      </div>
    </form>
  )
}

// typed attributes are entered as text; several values are separated by commas.
const typed = (attribute: Attribute) => !['status', 'select', 'member', 'reference'].includes(attribute.type)

// Chip is a property of the new record, named until it has a value.
const Chip = forwardRef<HTMLButtonElement, { icon?: ReactNode; label: ReactNode; empty: boolean } & React.ButtonHTMLAttributes<HTMLButtonElement>>(function Chip(
  { icon, label, empty, className, ...props },
  ref,
) {
  return (
    <Button ref={ref} {...props} size="sm" className={cn('max-w-56', empty ? 'text-ink-3' : 'text-ink-2', className)}>
      {icon}
      <span className="min-w-0 truncate">{label}</span>
    </Button>
  )
})

function Property({
  attribute,
  values,
  labels,
  members,
  onChange,
  onLabel,
}: {
  attribute: Attribute
  values: string[]
  labels: Record<string, Label>
  members: Member[]
  onChange: (values: string[]) => void
  onLabel: (id: string, label: Label) => void
}) {
  const toggle = (value: string) => onChange(attribute.multi ? (values.includes(value) ? values.filter((v) => v !== value) : [...values, value]) : [value])
  const empty = values.length === 0
  if (attribute.slug === 'context' && attribute.type === 'text') {
    return (
      <label className="mt-2 w-full text-[12px] text-ink-3">
        {attribute.name}
        <textarea
          value={values[0] ?? ''}
          rows={3}
          placeholder={contextHint}
          onChange={(e) => onChange(e.target.value ? [e.target.value] : [])}
          className="field-sizing-content mt-1 block max-h-80 min-h-20 w-full resize-y rounded-[var(--radius-control)] border border-border bg-transparent px-2 py-2 text-[13px] leading-5 text-ink outline-none placeholder:text-ink-3 focus:bg-bg"
        />
      </label>
    )
  }
  if (attribute.type === 'status') {
    return (
      <Picker
        trigger={<Chip empty={empty} icon={values[0] && <StageDot stage={values[0]} />} label={values[0] ?? attribute.name} />}
        placeholder={`Set ${attribute.name.toLowerCase()}…`}
        options={(attribute.options ?? []).map((o) => ({ value: o, label: o, icon: <StageDot stage={o} /> }))}
        selected={values}
        onSelect={toggle}
      />
    )
  }
  if (attribute.type === 'select') {
    return (
      <Picker
        trigger={<Chip empty={empty} label={empty ? attribute.name : <span className="flex gap-1">{values.map((v) => <ValueTag key={v} value={v} />)}</span>} className={cn(!empty && 'h-7 px-1')} />}
        placeholder={`Set ${attribute.name.toLowerCase()}…`}
        options={(attribute.options ?? []).map((o) => ({ value: o, label: o }))}
        selected={values}
        multiple={attribute.multi}
        onSelect={toggle}
      />
    )
  }
  if (attribute.type === 'member') {
    const member = (email: string) => members.find((m) => m.email === email)
    const chosen = values[0] && member(values[0])
    return (
      <Picker
        trigger={
          <Chip
            empty={empty}
            icon={chosen && <RecordIcon object="people" name={chosen.name || chosen.email} photo={chosen.photo} size={16} />}
            label={chosen ? (chosen.is_me ? `${chosen.name || chosen.email} (you)` : chosen.name || chosen.email) : attribute.name}
          />
        }
        placeholder={`Set ${attribute.name.toLowerCase()}…`}
        options={members.map((m) => ({ value: m.email, label: m.name || m.email, icon: <RecordIcon object="people" name={m.name || m.email} photo={m.photo} size={18} /> }))}
        selected={values}
        multiple={attribute.multi}
        onSelect={toggle}
      />
    )
  }
  if (attribute.type === 'reference') {
    return <ReferenceProperty attribute={attribute} values={values} labels={labels} toggle={toggle} onLabel={onLabel} />
  }
  return <TextProperty attribute={attribute} value={values[0] ?? ''} onChange={(text) => onChange(text ? [text] : [])} />
}

function ReferenceProperty({
  attribute,
  values,
  labels,
  toggle,
  onLabel,
}: {
  attribute: Attribute
  values: string[]
  labels: Record<string, Label>
  toggle: (value: string) => void
  onLabel: (id: string, label: Label) => void
}) {
  const [search, setSearch] = useState<string>()
  const target = attribute.target ?? ''
  const found = useRecords(target, useDebounced(search), 20).data?.records ?? []
  const first = labels[values[0]]
  return (
    <Picker
      trigger={
        <Chip
          empty={values.length === 0}
          icon={
            values.length > 0 && (
              <span className="flex -space-x-1">
                {values.slice(0, 3).map((id) => (
                  <RecordIcon key={id} object={target} name={labels[id]?.name ?? ''} photo={labels[id]?.photo} size={16} className="ring-1 ring-raised" />
                ))}
              </span>
            )
          }
          label={values.length === 0 ? attribute.name : values.length === 1 ? (first?.name ?? attribute.name) : `${values.length} ${attribute.name.toLowerCase()}`}
        />
      }
      placeholder={`Find ${target}…`}
      onSearch={setSearch}
      multiple={attribute.multi}
      selected={values}
      options={found.map((r) => ({ value: r.id, label: recordName(r), icon: <RecordIcon object={target} name={recordName(r)} photo={r.photo} size={16} /> }))}
      onSelect={(id) => {
        const record = found.find((r) => r.id === id)
        if (record) {
          onLabel(id, { name: recordName(record), photo: record.photo })
        }
        toggle(id)
      }}
    />
  )
}

const inputMode: Partial<Record<Attribute['type'], 'decimal' | 'email' | 'tel' | 'url'>> = { number: 'decimal', email: 'email', phone: 'tel', url: 'url', domain: 'url' }

// TextProperty is a chip that is also its own input: the attribute's name
// until something is typed, then what was typed.
function TextProperty({ attribute, value, onChange }: { attribute: Attribute; value: string; onChange: (text: string) => void }) {
  return (
    <label className="inline-flex h-6 items-center rounded-full border border-border px-2.5 text-[12px] transition-colors focus-within:border-ink-3/60 hover:bg-list-hover">
      <span className="sr-only">{attribute.name}</span>
      {attribute.type === 'number' && attribute.slug === 'value' && <span className="mr-1 text-ink-2">$</span>}
      <input
        value={value}
        type={attribute.type === 'date' ? 'date' : 'text'}
        inputMode={inputMode[attribute.type]}
        placeholder={attribute.multi ? attribute.name.replace(/s$/, '') : attribute.name}
        onChange={(e) => onChange(e.target.value)}
        className="field-sizing-content min-w-12 max-w-56 bg-transparent text-ink-2 outline-none placeholder:text-ink-3"
      />
    </label>
  )
}
