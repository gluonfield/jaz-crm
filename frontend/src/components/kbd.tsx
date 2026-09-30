import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export function Kbd({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <kbd
      className={cn(
        'ml-1 inline-flex h-[17px] min-w-[17px] items-center justify-center rounded-[4px] border border-border bg-bg px-1 font-sans text-[10.5px] font-medium leading-none text-ink-2',
        className,
      )}
    >
      {children}
    </kbd>
  )
}
