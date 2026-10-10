import { Link, useRouterState } from '@tanstack/react-router'
import { LoaderCircle } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

// NavItem is a sidebar link, indented by its depth in a tree.
export function NavItem({ to, icon, count, busy, depth = 0, children }: { to: string; icon: ReactNode; count?: number; busy?: string; depth?: number; children: ReactNode }) {
  const path = useRouterState({ select: (s) => s.location.pathname })
  const active = path === to || path.startsWith(to + '/')
  return (
    <Link
      to={to}
      style={depth ? { paddingLeft: 8 + depth * 14 } : undefined}
      className={cn(
        'flex h-7 items-center gap-2.5 rounded-[var(--radius-control)] px-2 font-medium text-ink-2 outline-none transition-colors duration-100 hover:bg-list-hover hover:text-ink focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:h-10 [&_svg]:size-4 [&_svg]:shrink-0',
        active && 'bg-list-active text-ink hover:bg-list-active',
      )}
    >
      {icon}
      <span className="flex-1 truncate">{children}</span>
      {!!count && <span className="text-[12px] tabular-nums text-ink-3">{count}</span>}
      {busy && (
        <span title={busy} className="flex text-ink-3">
          <LoaderCircle aria-label={busy} className="animate-spin motion-reduce:animate-none" />
        </span>
      )}
    </Link>
  )
}
