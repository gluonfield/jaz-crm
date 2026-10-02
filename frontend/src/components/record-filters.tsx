import { Bookmark, ChevronDown, ListFilter, Plus, Save, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useAction, useTool } from '@/lib/queries'
import type { CrmObject, RecordFilter, SavedFilter } from '@/lib/types'
import { Button } from '@jaz/ui/button'
import { inputClass } from './controls'
import { FilterCondition, needsValue } from './filter-condition'
import { Picker } from './picker'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover'

export function RecordFilters({ object, filters, query, selected, onChange, onApply }: { object: CrmObject; filters: RecordFilter[]; query: string; selected?: string; onChange: (filters: RecordFilter[]) => void; onApply: (filter?: SavedFilter) => void }) {
  const saved = useTool<{ filters: SavedFilter[] }>('list_saved_filters', { object: object.slug }, { refetchInterval: 5000 }).data?.filters ?? []
  const active = saved.find((f) => f.id === selected)
  const save = useAction<{ object: string; id?: string; name: string; query: string; filters: RecordFilter[] }, SavedFilter>('save_filter')
  const remove = useAction<{ id: string }>('delete_saved_filter')
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<RecordFilter[]>([])
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [name, setName] = useState('')
  const changed = active && ((active.query ?? '') !== query.trim() || JSON.stringify(active.filters) !== JSON.stringify(open || saving ? draft : filters))
  const incomplete = draft.some((f) => needsValue(f) && !f.value?.trim())
  const persist = (name: string, id?: string) => save.mutate({ object: object.slug, id, name, query, filters: draft }, { onSuccess: (filter) => {
    setOpen(false)
    setSaving(false)
    onApply(filter)
  } })
  return (
    <div className="flex min-w-0 items-center gap-1">
      <Picker
        trigger={<Button variant="ghost" className="min-w-0 max-w-40 shrink"><Bookmark className="shrink-0" /><span className="truncate">{active ? `${active.name}${changed ? ' · Edited' : ''}` : filters.length || query.trim() ? 'Custom filter' : `All ${object.name.toLowerCase()}`}</span><ChevronDown className="shrink-0" /></Button>}
        placeholder="Find saved filters…"
        options={[{ value: '', label: `All ${object.name.toLowerCase()}` }, ...saved.map((f) => ({ value: f.id, label: f.name }))]}
        selected={active ? [active.id] : filters.length || query.trim() ? [] : ['']}
        onSelect={(id) => onApply(saved.find((f) => f.id === id))}
      />
      <Popover open={open} onOpenChange={(next) => {
        if (next) {
          setDraft(filters)
        }
        setOpen(next)
      }}>
        <PopoverTrigger asChild><Button variant="ghost"><ListFilter />Filter{filters.length > 0 && <span className="tabular-nums text-ink-3">{filters.length}</span>}</Button></PopoverTrigger>
        <PopoverContent align="start" collisionPadding={8} className="w-[520px] max-w-[calc(100vw-2rem)] p-3" onKeyDown={(e) => e.stopPropagation()}>
          <div className="mb-2 flex items-center justify-between gap-2">
            <span className="text-[12px] text-ink-3">Match all conditions</span>
            {active && <Button variant="ghost" aria-label={`Delete saved filter ${active.name}`} onClick={() => {
              setOpen(false)
              setDeleting(true)
            }}><Trash2 /></Button>}
          </div>
          <div className="scrollbar-quiet grid max-h-72 gap-2 overflow-y-auto">
            {draft.map((filter, index) => <FilterCondition key={index} object={object} filter={filter} onChange={(next) => setDraft(draft.map((f, i) => i === index ? next : f))} onRemove={() => setDraft(draft.filter((_, i) => i !== index))} />)}
          </div>
          <div className="mt-2 flex items-center justify-between gap-2">
            <Picker
              trigger={<Button variant="ghost" disabled={draft.length >= 32}><Plus />Add condition</Button>}
              placeholder="Choose a field…"
              options={object.attributes.map((a) => ({ value: a.slug, label: a.name }))}
              onSelect={(attribute) => setDraft([...draft, { attribute, operator: 'is' }])}
            />
            <Button variant="primary" disabled={incomplete} onClick={() => {
              onChange(draft)
              setOpen(false)
            }}>Apply</Button>
          </div>
          {(active || draft.length > 0 || query.trim()) && <div className="mt-2 flex items-center gap-1 border-t border-border pt-2">
            <Button variant="ghost" disabled={incomplete} onClick={() => {
              setOpen(false)
              setName('')
              setSaving(true)
            }}><Bookmark />Save as…</Button>
            {changed && <Button variant="ghost" disabled={incomplete || save.isPending} onClick={() => persist(active.name, active.id)}><Save />Save changes</Button>}
          </div>}
        </PopoverContent>
      </Popover>
      <Dialog open={saving} onOpenChange={setSaving}>
        <DialogContent className="bg-raised text-ink sm:max-w-sm" aria-describedby={undefined}>
          <DialogTitle className="text-[14px]">Save filter</DialogTitle>
          <form autoComplete="off" className="grid gap-4" onSubmit={(e) => {
            e.preventDefault()
            persist(name)
          }}>
            <input autoFocus name="saved-filter-name" autoComplete="off" data-bwignore="true" aria-label="Filter name" placeholder="Filter name" maxLength={100} value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
            <div className="flex justify-end gap-2"><Button onClick={() => setSaving(false)}>Cancel</Button><Button type="submit" variant="primary" disabled={!name.trim() || save.isPending}>Save</Button></div>
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
