import { Button } from '@jaz/ui/button'
import { Link } from '@tanstack/react-router'
import { LoaderCircle, X } from 'lucide-react'
import { type ReactNode, useId } from 'react'
import type { Ref } from '@/lib/types'
import { cn } from '@/lib/utils'
import { RecordIcon } from './icons'

export const inputClass =
  'h-7 min-w-0 rounded-[var(--radius-control)] border border-border bg-bg px-2.5 text-[13px] text-ink outline-none placeholder:text-ink-3 focus:border-primary disabled:opacity-50 pointer-coarse:h-9 pointer-coarse:text-[16px]'

export function Header({ children, className }: { children?: ReactNode; className?: string }) {
  return (
    <header className={cn("flex h-11 shrink-0 items-center gap-2 border-b border-border px-4 text-[13px] font-medium text-ink [&>svg]:size-4 [&>svg]:shrink-0 [&>svg]:text-ink-2 max-md:[&>svg]:hidden", className)}>
      {children}
    </header>
  )
}

export function Loading() {
  return (
    <div role="status" aria-label="Loading" className="flex h-full flex-1 items-center justify-center text-ink-3">
      <LoaderCircle aria-hidden="true" className="size-5 animate-spin motion-reduce:animate-none" />
    </div>
  )
}

export function Tab({ active, children, onClick }: { active: boolean; children: ReactNode; onClick: () => void }) {
  return (
    <Button aria-pressed={active} onClick={onClick} className={active ? 'bg-list-active' : undefined}>
      {children}
    </Button>
  )
}

const chip = 'inline-flex min-h-[22px] min-w-0 max-w-full items-center gap-1.5 rounded-full border border-border bg-raised px-2 py-px text-[12px] text-ink-2'

export function Chip({ children, onRemove }: { children: ReactNode; onRemove?: () => void }) {
  const id = useId()
  return (
    <span className={chip}>
      <span id={id} className="flex min-w-0 items-center gap-1.5 truncate">
        {children}
      </span>
      {onRemove && (
        <button
          type="button"
          id={`${id}remove`}
          aria-label="Remove"
          aria-labelledby={`${id}remove ${id}`}
          onClick={(e) => {
            e.preventDefault()
            e.stopPropagation()
            onRemove()
          }}
          className="-mr-1 flex size-4 shrink-0 items-center justify-center rounded-full text-ink-3 outline-none hover:bg-list-active hover:text-ink"
        >
          <X className="size-3" />
        </button>
      )}
    </span>
  )
}

// RecordChip is a referenced record, opening it without acting on the row it
// sits in.
export function RecordChip({ object, value }: { object: string; value: Ref }) {
  const name = value.name || 'Unnamed'
  return (
    <Link to="/r/$recordId" params={{ recordId: value.id }} onClick={(e) => e.stopPropagation()}
      className={cn(chip, 'outline-none hover:text-ink focus-visible:border-primary')}>
      <RecordIcon object={object} name={name} photo={value.photo} icon={value.icon} size={14} />
      <span className="truncate">{name}</span>
    </Link>
  )
}

export function Section({ id, title, children }: { id?: string; title: string; children: ReactNode }) {
  return (
    <section id={id} className="mb-10">
      <h2 className="text-[14px] font-semibold text-ink">{title}</h2>
      <div className="mt-3 overflow-hidden rounded-[var(--radius-card)] border border-border bg-raised">{children}</div>
    </section>
  )
}

export function Row({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('flex min-h-14 items-center gap-3 border-b border-border/70 px-4 py-2.5 text-[13px] last:border-b-0', className)}>{children}</div>
}
