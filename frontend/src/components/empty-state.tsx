import type { ReactNode } from 'react'

export function EmptyState({ title, icon, children }: { title: string; icon: ReactNode; children?: ReactNode }) {
  return (
    <div className="flex h-full animate-rise flex-col items-center justify-center gap-3 px-6 pb-16 text-center">
      <div className="flex size-11 items-center justify-center rounded-xl border border-border bg-raised text-ink-3 shadow-xs [&_svg]:size-5">{icon}</div>
      <p className="text-[14px] font-medium text-ink">{title}</p>
      {children}
    </div>
  )
}
