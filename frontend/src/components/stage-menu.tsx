import { ArrowLeft, ArrowRight, Check, GripVertical, PanelLeftClose, Trash2 } from 'lucide-react'
import { type PointerEvent, useState } from 'react'
import { useAction } from '@/lib/queries'
import type { Attribute, CrmObject, StageEdit } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Button, inputClass } from './controls'
import { StageDot } from './stage'
import { Popover, PopoverContent, PopoverTrigger } from './ui/popover'

const actionClass = 'flex h-8 w-full items-center gap-2 rounded-[5px] px-2 text-[13px] text-ink-2 outline-none hover:bg-list-active hover:text-ink focus-visible:bg-list-active disabled:opacity-40 [&_svg]:size-3.5'

export function StageMenu({ object, status, stage, onCollapse, onPointerDown, disabled }: {
  object: CrmObject
  status: Attribute
  stage: string
  onCollapse: () => void
  onPointerDown: (event: PointerEvent<HTMLButtonElement>) => void
  disabled: boolean
}) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState(stage)
  const [deleting, setDeleting] = useState(false)
  const others = (status.options ?? []).filter((s) => s !== stage)
  const [replacement, setReplacement] = useState(others[0] ?? '')
  const edit = useAction<StageEdit>('edit_pipeline_stage')
  const pending = disabled || edit.isPending
  const options = status.options ?? []
  const index = options.indexOf(stage)
  const save = (change: Pick<StageEdit, 'action' | 'name' | 'before' | 'replacement'>) => edit.mutate(
    { object: object.slug, attribute: status.slug, stage, ...change },
    { onSuccess: () => setOpen(false) },
  )
  return (
    <Popover open={open} onOpenChange={(next) => {
      setOpen(next)
      if (next) {
        setName(stage)
        setDeleting(false)
        setReplacement(others[0] ?? '')
      }
    }}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={`Manage ${stage} stage`}
          title="Drag to reorder or click to edit"
          disabled={pending}
          onPointerDown={onPointerDown}
          className="flex size-6 shrink-0 touch-none cursor-grab items-center justify-center rounded-[var(--radius-control)] text-ink-3 outline-none hover:bg-list-active hover:text-ink focus-visible:ring-2 focus-visible:ring-ring active:cursor-grabbing disabled:opacity-40"
        ><GripVertical className="size-3.5" /></button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[272px] overflow-hidden rounded-[12px] border-border bg-raised p-0">
        {deleting ? (
          <form className="flex flex-col gap-3 p-3" onSubmit={(e) => {
            e.preventDefault()
            if (!pending && replacement) {
              save({ action: 'delete', replacement })
            }
          }}>
            <span className="truncate text-[13px] font-medium text-ink">Delete {stage}</span>
            <label className="flex flex-col gap-2 text-[12px] text-ink-2">
              Move deals to
              <select autoFocus aria-label="Replacement stage" value={replacement} onChange={(e) => setReplacement(e.target.value)} disabled={pending} className={cn(inputClass, 'h-8 w-full')}>
                {others.map((s) => <option key={s}>{s}</option>)}
              </select>
            </label>
            <span className="text-[12px] text-ink-3">Deals and their history are kept.</span>
            <div className="flex justify-end gap-1">
              <Button ghost onClick={() => setDeleting(false)}>Cancel</Button>
              <Button type="submit" disabled={pending || !replacement} className="border-destructive/20 bg-destructive/10 text-destructive hover:bg-destructive/20">Delete stage</Button>
            </div>
          </form>
        ) : (
          <>
            <form className="flex items-center gap-2 border-b border-border p-3" onSubmit={(e) => {
              e.preventDefault()
              if (!pending && name.trim() && name.trim() !== stage) {
                save({ action: 'rename', name: name.trim() })
              }
            }}>
              <StageDot stage={stage} />
              <input aria-label="Stage name" maxLength={80} value={name} onChange={(e) => setName(e.target.value)} disabled={pending} className={cn(inputClass, 'flex-1 focus:border-ink-3/40')} />
              <Button type="submit" ghost aria-label="Save stage name" disabled={pending || !name.trim() || name.trim() === stage} className="px-1.5"><Check /></Button>
            </form>
            <div className="p-1">
              <button type="button" className={actionClass} disabled={pending || index === 0} onClick={() => save({ action: 'move', before: options[index - 1] })}><ArrowLeft />Move left</button>
              <button type="button" className={actionClass} disabled={pending || index === options.length - 1} onClick={() => save({ action: 'move', before: options[index + 2] })}><ArrowRight />Move right</button>
              <button type="button" className={actionClass} onClick={() => {
                setOpen(false)
                onCollapse()
              }}><PanelLeftClose />Collapse stage</button>
              <button type="button" className={cn(actionClass, 'text-destructive hover:bg-destructive/10 hover:text-destructive')} disabled={pending || others.length === 0} title={others.length === 0 ? 'Keep at least one stage' : undefined} onClick={() => setDeleting(true)}><Trash2 />Delete stage</button>
            </div>
          </>
        )}
      </PopoverContent>
    </Popover>
  )
}
