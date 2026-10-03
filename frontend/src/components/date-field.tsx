import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Button } from '@jaz/ui/button'
import { CalendarDays } from 'lucide-react'
import { valueText, valuesOf } from '@/lib/crm'
import { dateValue, formatActionDate, isOverdue, zonedInput } from '@/lib/dates'
import { useAction, useWorkspace } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'
import { inputClass } from './controls'
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover'

export const dateMetadata = (slug: string) => ['action_date_basis', 'action_date_reason', 'action_date_source'].includes(slug)
const text = (record: CrmRecord, slug: string) => valuesOf(record, slug).map(valueText).join(', ')

export function DateLabel({ value, suggested = false }: { value: string; suggested?: boolean }) {
  const zone = useWorkspace()?.timezone ?? 'UTC'
  return <span className={cn('tabular-nums', isOverdue(value, zone) && 'text-ink')}>
    {formatActionDate(value, zone)}{suggested && <span className="ml-1.5 text-ink-3">Suggested</span>}
  </span>
}

// DateField shows a date and edits it in a popover; a prefix names it beside
// the date where nothing else does.
export function DateField({ record, slug = 'action_date', label = 'Action date', prefix }: { record: CrmRecord; slug?: string; label?: string; prefix?: string }) {
  const zone = useWorkspace()?.timezone ?? 'UTC'
  const value = text(record, slug)
  const basis = slug === 'action_date' ? text(record, 'action_date_basis') : ''
  const [open, setOpen] = useState(false)
  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger asChild>
      <Button variant="ghost" size={prefix ? 'default' : 'sm'} aria-label={label} onClick={(e) => e.stopPropagation()} className="max-w-full text-ink-2">
        {!value && <CalendarDays />}
        {value && prefix && (isOverdue(value, zone) ? <span className="text-danger">Overdue</span> : <span className="text-ink-3">{prefix}</span>)}
        {value ? <DateLabel value={value} suggested={basis === 'Suggested'} /> : label}
      </Button>
    </PopoverTrigger>
    <PopoverContent align="end" className="w-[300px]" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
      <DateEditor key={`${value}:${zone}`} record={record} slug={slug} label={label} value={value} zone={zone} close={() => setOpen(false)} />
    </PopoverContent>
  </Popover>
}

const oneDay = 86_400_000

// picks are the days people usually push a next step to, counted from today
// where the workspace is.
function picks(zone: string): [string, string][] {
  const today = Date.parse(`${zonedInput(new Date().toISOString(), zone).slice(0, 10)}T00:00:00Z`)
  const after = (days: number) => new Date(today + days * oneDay).toISOString().slice(0, 10)
  return [['Tomorrow', after(1)], ['Mon', after(((8 - new Date(today).getUTCDay()) % 7) || 7)], ['In 1 week', after(7)], ['In 2 weeks', after(14)]]
}

function DateEditor({ record, slug, label, value, zone, close }: { record: CrmRecord; slug: string; label: string; value: string; zone: string; close: () => void }) {
  const local = value.length > 10 ? zonedInput(value, zone) : value
  const [day, setDay] = useState(local.slice(0, 10))
  const [time, setTime] = useState(local.slice(11, 16))
  const [error, setError] = useState('')
  const actionDate = record.object === 'follow_ups' && slug === 'action_date'
  const write = useAction<object>(actionDate ? 'save_action_date' : 'upsert_record')
  const reason = slug === 'action_date' ? text(record, 'action_date_reason') : ''
  const source = slug === 'action_date' ? text(record, 'action_date_source') : ''
  const save = async (clear = false, on = day) => {
    try {
      const next = clear ? '' : dateValue(on, time, zone, value)
      await write.mutateAsync(actionDate ? { record_id: record.id, value: next } : { object: record.object, record_id: record.id, ...(next ? { values: { [slug]: next } } : { remove: { [slug]: [] } }) })
      close()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }
  return <div className="space-y-3" onKeyDown={(e) => {
    if (e.key === 'Enter' && e.target instanceof HTMLInputElement) {
      e.preventDefault()
      void save()
    }
  }}>
    <p className="text-[13px] font-semibold text-ink">{label}</p>
    <div className="flex flex-wrap gap-1.5">
      {picks(zone).map(([name, on]) => <button key={name} type="button" disabled={write.isPending} onClick={() => void save(false, on)} className="rounded-full bg-list-hover px-2.5 py-1 text-[12px] text-ink-2 outline-none hover:bg-list-active hover:text-ink focus-visible:ring-2 focus-visible:ring-ring">{name}</button>)}
    </div>
    <input type="date" aria-label="Date" value={day} onChange={(e) => setDay(e.target.value)} className={cn(inputClass, 'w-full')} />
    <label className="block text-[12px] text-ink-3">Time (optional) · {zone}<input type="time" aria-label="Time (optional)" value={time} disabled={!day} onChange={(e) => setTime(e.target.value)} className={cn(inputClass, 'mt-1 w-full')} /></label>
    {reason && <p className="rounded-[var(--radius-control)] bg-list-hover px-3 py-2 text-[12px] leading-[1.5] text-ink-2">{text(record, 'action_date_basis')}{value && ` ${formatActionDate(value, zone)}`}: {reason}</p>}
    {source && <Link to="/i/$interactionId" params={{ interactionId: source }} className="block text-[12px] text-ink-2 underline">View source conversation</Link>}
    {error && <p role="alert" className="text-[12px] text-danger">{error}</p>}
    <div className="flex justify-between gap-2">
      <Button variant="ghost" disabled={write.isPending} onClick={() => void save(true)}>Clear</Button>
      <Button disabled={write.isPending} onClick={() => void save()}>Save</Button>
    </div>
  </div>
}
