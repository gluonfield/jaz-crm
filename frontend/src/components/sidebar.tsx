import { Link, useRouterState } from '@tanstack/react-router'
import { ChevronDown, Inbox, LogOut, Monitor, Moon, PlugZap, Search, Settings, Sun, TextSearch } from 'lucide-react'
import type { ReactNode } from 'react'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { signOut, useMe } from '@/lib/account'
import { embedded } from '@/lib/api'
import { useObjects, useTool, useWorkspace } from '@/lib/queries'
import { setSchemePreference } from '@/lib/theme'
import type { Contact } from '@/lib/types'
import { setUI } from '@/lib/ui'
import { cn } from '@/lib/utils'
import { ObjectIcon, RecordIcon } from './icons'
import { Kbd } from './kbd'

export function Sidebar() {
  const objects = useObjects() ?? []
  const pending = useTool<{ contacts: Contact[] }>('list_triage', { status: 'pending', limit: 200 }).data?.contacts.length ?? 0
  return (
    <aside className="flex w-[232px] shrink-0 flex-col gap-px px-2.5 pb-3 pt-2.5 text-[13px]">
      <div className="mb-2 flex items-center gap-1">
        <WorkspaceMenu />
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              onClick={() => setUI({ paletteOpen: true })}
              aria-label="Search"
              className="flex size-7 shrink-0 items-center justify-center rounded-[var(--radius-control)] text-ink-2 outline-none transition-colors hover:bg-list-hover hover:text-ink"
            >
              <Search className="size-4" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="bottom">
            Search <Kbd>⌘K</Kbd>
          </TooltipContent>
        </Tooltip>
      </div>
      <NavItem to="/triage" icon={<Inbox />} count={pending}>
        Triage
      </NavItem>
      <NavItem to="/search" icon={<TextSearch />}>
        Conversations
      </NavItem>
      <Section title="Records">
        {objects.map((o) => (
          <NavItem key={o.slug} to={`/o/${o.slug}`} icon={<ObjectIcon slug={o.slug} />}>
            {o.name}
          </NavItem>
        ))}
      </Section>
      <Section title="Workspace">
        <NavItem to="/connections" icon={<PlugZap />}>
          Connections
        </NavItem>
        <NavItem to="/settings" icon={<Settings />}>
          Settings
        </NavItem>
      </Section>
    </aside>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="mt-4 flex flex-col gap-px">
      <div className="flex h-7 items-center px-2 text-[12px] font-medium text-ink-3">{title}</div>
      {children}
    </div>
  )
}

function NavItem({ to, icon, count, children }: { to: string; icon: ReactNode; count?: number; children: ReactNode }) {
  const path = useRouterState({ select: (s) => s.location.pathname })
  const active = path === to || path.startsWith(to + '/')
  return (
    <Link
      to={to}
      className={cn(
        'flex h-7 items-center gap-2.5 rounded-[var(--radius-control)] px-2 font-medium text-ink-2 outline-none transition-colors duration-100 hover:bg-list-hover hover:text-ink focus-visible:ring-2 focus-visible:ring-ring [&_svg]:size-4 [&_svg]:shrink-0',
        active && 'bg-list-active text-ink hover:bg-list-active',
      )}
    >
      {icon}
      <span className="flex-1 truncate">{children}</span>
      {!!count && <span className="text-[12px] tabular-nums text-ink-3">{count}</span>}
    </Link>
  )
}

function WorkspaceMenu() {
  const workspace = useWorkspace()
  const me = useMe()
  const name = workspace?.name ?? ''
  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="flex h-7 min-w-0 flex-1 select-none items-center gap-2 rounded-[var(--radius-control)] px-1.5 font-semibold text-ink outline-none hover:bg-list-hover data-[state=open]:bg-list-active">
        {name && <RecordIcon object="companies" name={name} />}
        <span className="truncate">{name}</span>
        <ChevronDown className="size-3 shrink-0 text-ink-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-56">
        {me && <DropdownMenuLabel className="truncate text-[12px] font-normal text-ink-3">{me.email}</DropdownMenuLabel>}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Sun /> Theme
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuItem onSelect={() => setSchemePreference('light')}>
              <Sun /> Light
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setSchemePreference('dark')}>
              <Moon /> Dark
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setSchemePreference('system')}>
              <Monitor /> System
            </DropdownMenuItem>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuItem asChild>
          <Link to="/settings">
            <Settings /> Settings
          </Link>
        </DropdownMenuItem>
        {!embedded() && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => void signOut()}>
              <LogOut /> Sign out
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
