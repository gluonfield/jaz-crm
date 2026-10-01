import { Bookmark, ChevronDown, ListFilter, Plus, Save, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useAction, useTool } from '@/lib/queries'
import type { CrmObject, RecordFilter, SavedFilter } from '@/lib/types'
import { Button, Chip, inputClass } from './controls'
import { FilterCondition, FilterLabel, needsValue } from './filter-condition'
import { Picker } from './picker'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover'

export function RecordFilters({ object, filters, query, selected, onChange, onApply }: { object: CrmObject; filters: RecordFilter[]; query: string; selected?: string; onChange: (filters: RecordFilter[]) => void; onApply: (filter?: SavedFilter) => void }) {
  const saved = useTool<{ filters: SavedFilter[] }>('list_saved_filters', { object: object.slug }).data?.filters ?? []
  const active = saved.find((f) => f.id === selected)
  const changed = active && ((active.query ?? '') !== query.trim() || JSON.stringify(active.filters) !== JSON.stringify(filters))
  const save = useAction<{ object: string; id?: string; name: string; query: string; filters: RecordFilter[] }, SavedFilter>('save_filter')
  const remove = useAction<{ id: string }>('delete_saved_filter')
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<RecordFilter[]>([])
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [name, setName] = useState('')
  const persist = (name: string, id?: string) => save.mutate({ object: object.slug, id, name, query, filters }, { onSuccess: (filter) => {
    setSaving(false)
    onApply(filter)
  } })
  return (
    <div className="flex shrink-0 flex-wrap items-center gap-1.5 border-b border-border px-4 py-1.5 text-[13px]">
      <Picker
        trigger={<Button ghost className="max-w-56"><Bookmark /><span className="truncate">{active ? `${active.name}${changed ? ' · Edited' : ''}` : filters.length || query.trim() ? 'Custom filter' : `All ${object.name.toLowerCase()}`}</span><ChevronDown /></Button>}
        placeholder="Find saved filters…"
        options={[{ value: '', label: `All ${object.name.toLowerCase()}` }, ...saved.map((f) => ({ value: f.id, label: f.name }))]}
        selected={active ? [active.id] : filters.length || query.trim() ? [] : ['']}
        onSelect={(id) => onApply(saved.find((f) => f.id === id))}
      />
      <Popover open={open} onOpenChange={(next) => {
        setDraft(filters)
        setOpen(next)
      }}>
        <PopoverTrigger asChild><Button ghost><ListFilter />Filter{filters.length > 0 && <span className="tabular-nums text-ink-3">{filters.length}</span>}</Button></PopoverTrigger>
        <PopoverContent align="start" collisionPadding={8} className="w-[520px] max-w-[calc(100vw-2rem)] p-3" onKeyDown={(e) => e.stopPropagation()}>
          <div className="mb-2 text-[12px] text-ink-3">Match all conditions</div>
          <div className="scrollbar-quiet grid max-h-72 gap-2 overflow-y-auto">
            {draft.map((filter, index) => <FilterCondition key={index} object={object} filter={filter} onChange={(next) => setDraft(draft.map((f, i) => i === index ? next : f))} onRemove={() => setDraft(draft.filter((_, i) => i !== index))} />)}
          </div>
          <div className="mt-2 flex items-center justify-between gap-2">
            <Picker
              trigger={<Button ghost disabled={draft.length >= 32}><Plus />Add condition</Button>}
              placeholder="Choose a field…"
              options={object.attributes.map((a) => ({ value: a.slug, label: a.name }))}
              onSelect={(attribute) => setDraft([...draft, { attribute, operator: 'is' }])}
            />
            <Button primary disabled={draft.some((f) => needsValue(f) && !f.value?.trim())} onClick={() => {
              onChange(draft)
              setOpen(false)
            }}>Apply</Button>
          </div>
        </PopoverContent>
      </Popover>
      {filters.map((filter, index) => <Chip key={index} onRemove={() => onChange(filters.filter((_, i) => i !== index))}><FilterLabel object={object} filter={filter} /></Chip>)}
      {(filters.length > 0 || query.trim()) && <Button ghost onClick={() => {
        setName('')
        setSaving(true)
      }}><Bookmark />Save as…</Button>}
      {changed && <Button ghost disabled={save.isPending} onClick={() => persist(active.name, active.id)}><Save />Save changes</Button>}
      {active && <Button ghost aria-label={`Delete saved filter ${active.name}`} onClick={() => setDeleting(true)}><Trash2 /></Button>}
      <Dialog open={saving} onOpenChange={setSaving}>
        <DialogContent className="bg-raised text-ink sm:max-w-sm" aria-describedby={undefined}>
          <DialogTitle className="text-[14px]">Save filter</DialogTitle>
          <form autoComplete="off" className="grid gap-4" onSubmit={(e) => {
            e.preventDefault()
            persist(name)
          }}>
            <input autoFocus name="saved-filter-name" autoComplete="off" data-bwignore="true" aria-label="Filter name" placeholder="Filter name" maxLength={100} value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
            <div className="flex justify-end gap-2"><Button onClick={() => setSaving(false)}>Cancel</Button><Button type="submit" primary disabled={!name.trim() || save.isPending}>Save</Button></div>
          </form>
        </DialogContent>
      </Dialog>
      <Dialog open={deleting} onOpenChange={setDeleting}>
        <DialogContent className="bg-raised text-ink sm:max-w-sm" aria-describedby={undefined}>
          <DialogTitle className="text-[14px]">Delete “{active?.name}” filter?</DialogTitle>
          <div className="flex justify-end gap-2">
            <Button onClick={() => setDeleting(false)}>Cancel</Button>
            <Button disabled={remove.isPending} onClick={() => active && remove.mutate({ id: active.id }, { onSuccess: () => {
              setDeleting(false)
              onApply()
            } })}>Delete</Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
