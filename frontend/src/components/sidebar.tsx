import { Link } from '@tanstack/react-router'
import { Check, ChevronDown, Inbox, LogOut, Monitor, Moon, PlugZap, Plus, Search, Settings, Sun, Trash2 } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { DialogClose } from '@/components/ui/dialog'
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
import { steps, syncing, useConnections } from '@/lib/sync'
import { setSchemePreference } from '@/lib/theme'
import type { Contact, CrmObject, TrashedRecord } from '@/lib/types'
import { setUI } from '@/lib/ui'
import { cn } from '@/lib/utils'
import { useMoveWorkspace, useWorkspaces } from '@/lib/workspaces'
import { DataNav } from './data-nav'
import { ObjectIcon, RecordIcon } from './icons'
import { NavItem } from './nav-item'
import { Kbd } from './kbd'
import { NameDialog } from './prompt-dialog'

export function Sidebar({ className }: { className?: string }) {
  const objects = useObjects() ?? []
  const pending = useTool<{ contacts: Contact[] }>('list_triage', { status: 'pending', limit: 200 }).data?.contacts.length ?? 0
  const trash = useTool<{ records: TrashedRecord[] }>('list_trash').data?.records.length ?? 0
  const sync = useConnections()?.connections.find(syncing)
  const records = objects.filter((o) => o.standard && o.slug !== 'pages')
  const nav = (o: CrmObject) => (
    <NavItem key={o.slug} to={`/o/${o.slug}`} icon={<ObjectIcon slug={o.slug} />}>
      {o.name}
    </NavItem>
  )
  return (
    <aside className={cn('flex w-[232px] shrink-0 flex-col gap-px px-2.5 pb-3 pt-2.5 text-[13px]', className)}>
      <div className="mb-2 flex items-center gap-1">
        <WorkspaceMenu />
        <Tooltip>
          <TooltipTrigger asChild>
            {/* Opening search from the navigation drawer closes the drawer. */}
            <DialogClose asChild>
              <button
                onClick={() => setUI({ paletteOpen: true })}
                aria-label="Search"
                className="flex size-7 shrink-0 items-center justify-center rounded-[var(--radius-control)] text-ink-2 outline-none transition-colors hover:bg-list-hover hover:text-ink pointer-coarse:size-10"
              >
                <Search className="size-4" />
              </button>
            </DialogClose>
          </TooltipTrigger>
          <TooltipContent side="bottom">
            Search <Kbd>⌘K</Kbd>
          </TooltipContent>
        </Tooltip>
      </div>
      <NavItem to="/triage" icon={<Inbox />} count={pending}>
        Triage
      </NavItem>
      {records.filter((o) => o.slug === 'follow_ups').map(nav)}
      <Section title="Records">{records.filter((o) => o.slug !== 'follow_ups').map(nav)}</Section>
      <DataNav />
      <Section title="Workspace">
        <NavItem to="/connections" icon={<PlugZap />} busy={sync && (steps[sync.step ?? ''] ?? steps.GmailBackfill)}>
          Connections
        </NavItem>
        <NavItem to="/settings" icon={<Settings />}>
          Settings
        </NavItem>
        {trash > 0 && <NavItem to="/trash" icon={<Trash2 />} count={trash}>Trash</NavItem>}
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

function WorkspaceMenu() {
  const workspace = useWorkspace()
  const me = useMe()
  const workspaces = useWorkspaces()
  const move = useMoveWorkspace()
  const [creating, setCreating] = useState(false)
  const name = workspace?.name ?? ''
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger className="flex h-7 min-w-0 flex-1 select-none items-center gap-2 rounded-[var(--radius-control)] px-1.5 font-semibold text-ink outline-none hover:bg-list-hover data-[state=open]:bg-list-active pointer-coarse:h-10">
          {name && <RecordIcon object="companies" name={name} />}
          <span className="truncate">{name}</span>
          <ChevronDown className="size-3 shrink-0 text-ink-3" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-56">
          {me && <DropdownMenuLabel>{me.email}</DropdownMenuLabel>}
          {workspaces.map((w) => (
            <DropdownMenuItem key={w.id} onSelect={() => !w.default && move.mutate({ workspace_id: w.id })}>
              <RecordIcon object="companies" name={w.name} size={16} />
              <span className="flex-1 truncate">{w.name}</span>
              {w.default && <Check aria-label="Current" />}
            </DropdownMenuItem>
          ))}
          <DropdownMenuItem onSelect={() => setCreating(true)}>
            <Plus /> New workspace
          </DropdownMenuItem>
          <DropdownMenuSeparator />
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
      <NameDialog open={creating} onOpenChange={setCreating} title="New workspace" placeholder="Name, such as Acme" action="Create" onSubmit={(name) => move.mutate({ name })} />
    </>
  )
}
