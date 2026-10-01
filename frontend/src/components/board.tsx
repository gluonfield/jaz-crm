import { useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { type KeyboardEvent, type PointerEvent, type ReactNode, useState } from 'react'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { formatNumber } from '@/lib/format'
import { useAction } from '@/lib/queries'
import type { Attribute, CrmObject, CrmRecord, Ref, StageEdit } from '@/lib/types'
import { useColumnDrag } from '@/lib/use-column-drag'
import { cn } from '@/lib/utils'
import { Button, inputClass } from './controls'
import { CreateRecord, singular } from './create-record'
import { RecordIcon } from './icons'
import { RecordMenu } from './record-menu'
import { StageDot } from './stage'
import { StageMenu } from './stage-menu'
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover'

// Board lays an object's records out in columns by the stage of its status,
// and moves a record to another stage when its card is dropped there.
export function Board({ object, status, records }: { object: CrmObject; status: Attribute; records: CrmRecord[] }) {
  const upsert = useAction<object>('upsert_record')
  const [moved, setMoved] = useState<Record<string, { from: string; to: string }>>({})
  const [dragging, setDragging] = useState<string>()
  const [over, setOver] = useState<string>()
  const [collapsed, setCollapsed] = useState<string[]>([])
  const edit = useAction<StageEdit>('edit_pipeline_stage')
  const { boardRef, order, view: columnDrag, start: dragColumn } = useColumnDrag(status.options ?? [], (stage, before) => edit.mutateAsync({ object: object.slug, attribute: status.slug, action: 'move', stage, before }))
  const amount = object.attributes.find((a) => a.type === 'number')
  const saved = (r: CrmRecord) => valueText(valuesOf(r, status.slug)[0] ?? '')
  // A moved card shows its new stage until the saved stage changes.
  const stageOf = (r: CrmRecord) => (moved[r.id]?.from === saved(r) ? moved[r.id].to : saved(r))
  const move = (id: string, stage: string) => {
    const record = records.find((r) => r.id === id)
    if (!record || stageOf(record) === stage) {
      return
    }
    setMoved((current) => ({ ...current, [id]: { from: saved(record), to: stage } }))
    upsert.mutate(
      { object: object.slug, record_id: id, values: { [status.slug]: stage } },
      {
        onError: () =>
          setMoved((current) => {
            const rest = { ...current }
            delete rest[id]
            return rest
          }),
      },
    )
  }
  return (
    <div ref={boardRef} className={cn('scrollbar-quiet flex min-h-0 flex-1 gap-3 overflow-x-auto p-3 pt-3', columnDrag && 'select-none')}>
      {order.map((stage) => {
        const folded = collapsed.includes(stage)
        const cards = records.filter((r) => stageOf(r) === stage)
        const total = amount ? cards.reduce((sum, r) => sum + (Number(valueText(valuesOf(r, amount.slug)[0] ?? '0')) || 0), 0) : 0
        return (
          <section
            key={stage}
            aria-label={stage}
            data-stage={stage}
            style={columnDrag ? { transform: `translateX(${columnDrag.offsets[stage]}px)` } : undefined}
            onDragOver={(e) => {
              if (dragging) {
                e.preventDefault()
                e.dataTransfer.dropEffect = 'move'
                setOver(stage)
              }
            }}
            onDragLeave={(e) => {
              if (!e.currentTarget.contains(e.relatedTarget as Node)) {
                setOver(undefined)
              }
            }}
            onDrop={(e) => {
              e.preventDefault()
              setDragging(undefined)
              setOver(undefined)
              move(e.dataTransfer.getData('text/plain'), stage)
            }}
            className={cn(
              'flex shrink-0 flex-col rounded-[var(--radius-card)] transition-[background-color,box-shadow] duration-150',
              folded ? 'w-12 bg-list-hover' : 'w-[272px]',
              dragging && 'bg-list-hover',
              columnDrag && 'transition-transform duration-150 ease-out motion-reduce:transition-none',
              columnDrag?.stage === stage && 'bg-list-hover outline-1 -outline-offset-1 outline-dashed outline-ink-3/40 [&>*]:invisible',
              over === stage && 'bg-primary-soft shadow-[inset_0_0_0_1px_var(--color-primary)]',
            )}
          >
            {folded ? (
              <button type="button" aria-label={`Expand ${stage} stage`} onClick={() => setCollapsed((current) => current.filter((s) => s !== stage))} className="flex h-full flex-col items-center gap-3 rounded-[var(--radius-card)] py-3 text-[12px] text-ink-2 outline-none hover:bg-list-active focus-visible:ring-2 focus-visible:ring-ring">
                <StageDot stage={stage} />
                <span className="max-h-48 truncate font-medium [writing-mode:vertical-rl]">{stage}</span>
                <span className="tabular-nums text-ink-3">{cards.length}</span>
              </button>
            ) : <Column object={object} status={status} stage={stage} count={cards.length} amount={amount} total={amount && total ? total : undefined} disabled={edit.isPending}
              onCollapse={() => setCollapsed((current) => [...current, stage])}
              onPointerDown={(e) => dragColumn(stage, e)}>
              {cards.map((r) => (
                <Card key={r.id} record={r} amount={amount} dragging={dragging === r.id} onDrag={setDragging} />
              ))}
            </Column>}
          </section>
        )
      })}
      <NewStage object={object} status={status} />
    </div>
  )
}

function NewStage({ object, status }: { object: CrmObject; status: Attribute }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const add = useAction<{ object: string; attribute: string; value: string }>('add_attribute_option')
  const ready = !!name.trim() && !add.isPending
  return (
    <Popover open={open} onOpenChange={(next) => {
      setOpen(next)
      if (next) {
        setName('')
      }
    }}>
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={add.isPending}
          className="flex h-9 shrink-0 items-center gap-1.5 self-start rounded-[var(--radius-control)] border border-dashed border-border px-3 text-[12.5px] text-ink-3 outline-none transition-colors hover:bg-list-hover hover:text-ink focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
        >
          <Plus className="size-3.5" /> Add stage
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-[272px] rounded-[12px] border-border bg-raised p-3">
        <form className="flex gap-2" onSubmit={(e) => {
          e.preventDefault()
          if (ready) {
            add.mutate({ object: object.slug, attribute: status.slug, value: name.trim() }, { onSuccess: () => setOpen(false) })
          }
        }}>
          <input
            aria-label="Stage name"
            placeholder="Stage name"
            maxLength={80}
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={add.isPending}
            className={cn(inputClass, 'flex-1')}
          />
          <Button type="submit" primary disabled={!ready}>Add</Button>
        </form>
      </PopoverContent>
    </Popover>
  )
}

function Column({
  object,
  status,
  stage,
  count,
  amount,
  total,
  children,
  disabled,
  onCollapse,
  onPointerDown,
}: {
  object: CrmObject
  status: Attribute
  stage: string
  count: number
  amount?: Attribute
  total?: number
  children: ReactNode
  disabled: boolean
  onCollapse: () => void
  onPointerDown: (event: PointerEvent<HTMLButtonElement>) => void
}) {
  const [adding, setAdding] = useState(false)
  const kind = singular(object)
  return (
    <>
      <header className="flex h-9 shrink-0 items-center gap-2 px-3 text-[13px]">
        <StageDot stage={stage} className="shrink-0" />
        <span className="min-w-0 truncate font-medium text-ink" title={stage}>{stage}</span>
        <span className="shrink-0 tabular-nums text-ink-3">{count}</span>
        {total !== undefined && (
          <span className="ml-auto shrink-0 tabular-nums text-ink-3" title={formatNumber(total, amount?.slug)}>
            {formatNumber(total, amount?.slug, true)}
          </span>
        )}
        <button
          type="button"
          aria-label={`New ${kind} in ${stage}`}
          onClick={() => setAdding(true)}
          className={cn(
            'flex size-6 shrink-0 items-center justify-center rounded-[var(--radius-control)] text-ink-3 outline-none transition-colors hover:bg-list-active hover:text-ink focus-visible:ring-2 focus-visible:ring-ring',
            total === undefined && 'ml-auto',
          )}
        >
          <Plus className="size-3.5" />
        </button>
        <StageMenu object={object} status={status} stage={stage} disabled={disabled} onCollapse={onCollapse} onPointerDown={onPointerDown} />
      </header>
      <ol className="scrollbar-quiet flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto px-2 pb-2">
        {children}
        <li>
          <button
            type="button"
            onClick={() => setAdding(true)}
            className="flex h-8 w-full items-center gap-1.5 rounded-[var(--radius-control)] px-2 text-[12.5px] text-ink-3 outline-none transition-colors hover:bg-list-active hover:text-ink-2 focus-visible:ring-2 focus-visible:ring-ring"
          >
            <Plus className="size-3.5" /> New {kind}
          </button>
        </li>
      </ol>
      <CreateRecord object={object} open={adding} onOpenChange={setAdding} initial={{ [status.slug]: stage }} />
    </>
  )
}

function Card({ record, amount, dragging, onDrag }: { record: CrmRecord; amount?: Attribute; dragging: boolean; onDrag: (id?: string) => void }) {
  const navigate = useNavigate()
  const company = valuesOf(record, 'company')[0] as Ref | undefined
  const people = valuesOf(record, 'people') as Ref[]
  const value = amount && valuesOf(record, amount.slug)[0]
  const open = () => navigate({ to: '/r/$recordId', params: { recordId: record.id } })
  return (
    <li>
      <RecordMenu record={record}>
        <div
          role="link"
          tabIndex={0}
          draggable
          onDragStart={(e) => {
            e.dataTransfer.setData('text/plain', record.id)
            e.dataTransfer.effectAllowed = 'move'
            onDrag(record.id)
          }}
          onDragEnd={() => onDrag(undefined)}
          onClick={open}
          onKeyDown={(e: KeyboardEvent) => e.key === 'Enter' && open()}
          className={cn(
            'group flex cursor-grab flex-col gap-2 rounded-[8px] border border-border bg-raised px-3 py-2.5 text-[13px] shadow-xs outline-none transition-[border-color,box-shadow,opacity] duration-150 hover:border-ink-3/40 hover:shadow-sm data-[state=open]:border-ink-3/40 focus-visible:ring-2 focus-visible:ring-ring active:cursor-grabbing',
            dragging && 'opacity-40',
          )}
        >
          <span className="font-medium leading-snug text-ink">{recordName(record)}</span>
          {company && (
            <span className="flex min-w-0 items-center gap-1.5 text-[12px] text-ink-2">
              <RecordIcon object="companies" name={company.name ?? ''} photo={company.photo} size={14} />
              <span className="truncate">{company.name}</span>
            </span>
          )}
          {(value || people.length > 0) && (
            <span className="flex items-center gap-2">
              {value && <span className="tabular-nums text-ink">{formatNumber(valueText(value), amount?.slug)}</span>}
              <span className="ml-auto flex -space-x-1">
                {people.slice(0, 4).map((p) => (
                  <RecordIcon key={p.id} object="people" name={p.name ?? ''} photo={p.photo} size={18} className="ring-2 ring-raised" />
                ))}
              </span>
            </span>
          )}
        </div>
      </RecordMenu>
    </li>
  )
}
