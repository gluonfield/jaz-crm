import { X } from 'lucide-react'
import { type ComponentProps, type ReactNode, useId } from 'react'
import { cn } from '@/lib/utils'

export const inputClass =
  'h-7 min-w-0 rounded-[var(--radius-control)] border border-border bg-bg px-2.5 text-[13px] text-ink outline-none placeholder:text-ink-3 focus:border-primary disabled:opacity-50'

export function Header({ children }: { children?: ReactNode }) {
  return (
    <header className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-4 text-[13px] font-medium text-ink [&>svg]:size-4 [&>svg]:shrink-0 [&>svg]:text-ink-2">
      {children}
    </header>
  )
}

export function Button({ primary, className, ...props }: ComponentProps<'button'> & { primary?: boolean }) {
  return (
    <button
      type="button"
      {...props}
      className={cn(
        'inline-flex h-7 shrink-0 items-center gap-1.5 rounded-[var(--radius-control)] border px-2.5 text-[12.5px] font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50 [&_svg]:size-3.5',
        primary ? 'border-primary bg-primary text-on-primary hover:bg-primary-strong' : 'border-border text-ink hover:bg-list-hover',
        className,
      )}
    />
  )
}

export function Tab({ active, children, onClick }: { active: boolean; children: ReactNode; onClick: () => void }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        'inline-flex h-[26px] items-center gap-1.5 rounded-[var(--radius-control)] border px-2.5 text-[12.5px] font-medium outline-none transition-colors duration-100 [&_svg]:size-3.5',
        active ? 'border-border bg-list-active text-ink' : 'border-transparent text-ink-2 hover:bg-list-hover hover:text-ink',
      )}
    >
      {children}
    </button>
  )
}

export function Chip({ children, onRemove }: { children: ReactNode; onRemove?: () => void }) {
  const id = useId()
  return (
    <span className="inline-flex h-[22px] min-w-0 max-w-full items-center gap-1.5 rounded-full border border-border bg-raised px-2 text-[12px] text-ink-2">
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
