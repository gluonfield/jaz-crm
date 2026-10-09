import { ChevronDown, UserRound } from 'lucide-react'
import { Button } from '@jaz/ui/button'
import { useWorkspace } from '@/lib/queries'
import type { RecordFilter } from '@/lib/types'
import { Picker } from './picker'

// OwnerFilter chooses whose records show; compact names them in a word.
export function OwnerFilter({ filters, compact = false, onChange }: { filters: RecordFilter[]; compact?: boolean; onChange: (filters: RecordFilter[]) => void }) {
  const members = useWorkspace()?.members ?? []
  const owners = filters.filter((f) => f.attribute === 'owner')
  const owner = owners.length === 1 && owners[0].operator === 'is' ? owners[0].value : undefined
  const options = [
    { value: '', label: 'Everyone' },
    ...members.map((m) => ({ value: m.email, label: m.is_me ? 'Assigned to me' : m.name || m.email })),
  ]
  const selected = owners.length === 0 ? '' : owner
  const label = options.find((o) => o.value === selected)?.label ?? 'Owner filter'
  const short = selected === '' ? 'Everyone' : members.find((m) => m.email === selected)?.is_me ? 'Mine' : label.split(' ')[0]
  return (
    <Picker
      trigger={compact
        ? <Button variant="ghost" aria-label={`Filter by owner: ${label}`} className="min-w-0 max-w-28 shrink-0 text-ink-2"><span className="truncate">{short}</span><ChevronDown className="text-ink-3" /></Button>
        : <Button variant="ghost" aria-label={`Filter by owner: ${label}`} className="min-w-0 max-w-full shrink-0 bg-list-hover hover:bg-list-active"><UserRound /><span className="max-w-28 truncate">{label}</span><ChevronDown /></Button>}
      placeholder="Filter by owner…"
      align={compact ? 'end' : 'start'}
      options={options}
      selected={selected === undefined ? [] : [selected]}
      onSelect={(value) => {
        const next = filters.filter((f) => f.attribute !== 'owner')
        if (value) {
          next.push({ attribute: 'owner', operator: 'is', value })
        }
        onChange(next)
      }}
    />
  )
}
