import type { ReactNode } from 'react'
import { useConnect } from '@/lib/sync'
import { Button } from './controls'

// EmptyState says what will appear on an empty page, and when.
export function EmptyState({ title, icon, children, action }: { title: string; icon: ReactNode; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex h-full animate-rise flex-col items-center justify-center gap-3 px-6 pb-16 text-center">
      <div className="flex size-11 items-center justify-center rounded-xl border border-border bg-raised text-ink-3 shadow-xs [&_svg]:size-5">{icon}</div>
      <div className="flex max-w-[360px] flex-col gap-1">
        <p className="text-[14px] font-medium text-ink">{title}</p>
        {children && <p className="text-[13px] leading-[1.5] text-ink-3">{children}</p>}
      </div>
      {action}
    </div>
  )
}

export function ConnectGoogle() {
  const connect = useConnect()
  return (
    connect && (
      <Button primary className="mt-1" onClick={connect}>
        Connect Google
      </Button>
    )
  )
}
