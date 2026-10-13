import { useNavigate } from '@tanstack/react-router'
import { FileText, Table2 } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { call } from '@/lib/api'
import { slugify } from '@/lib/crm'
import { useAction } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { NameDialog } from './prompt-dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './ui/dropdown-menu'

export function useNewPage() {
  const navigate = useNavigate()
  const create = useAction<object, { record: CrmRecord }>('upsert_record')
  return (parent?: string) => create.mutate(
    { object: 'pages', values: { name: 'Untitled', ...(parent && { parent }) } },
    { onSuccess: (out) => void navigate({ to: '/r/$recordId', params: { recordId: out.record.id } }) },
  )
}

export function NewTable({ parent, open, onOpenChange }: { parent?: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const navigate = useNavigate()
  const create = useAction<object, { slug: string }>('create_object')
  return <NameDialog
    open={open}
    onOpenChange={onOpenChange}
    title="New Database"
    placeholder="Name, such as Painpoints"
    action="Create"
    onSubmit={(name) => create.mutate(
      { slug: slugify(name), name, parent },
      { onSuccess: async (table) => {
        await call('upsert_record', { object: table.slug, values: { name: 'Untitled' } })
        void navigate({ to: '/o/$object', params: { object: table.slug } })
      } },
    )}
  />
}

export function NewData({ parent, children }: { parent?: string; children: ReactNode }) {
  const newPage = useNewPage()
  const [naming, setNaming] = useState(false)
  return <>
    <DropdownMenu>
      <DropdownMenuTrigger asChild>{children}</DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-44">
        <DropdownMenuItem onSelect={() => newPage(parent)}><FileText /> Page</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => setNaming(true)}><Table2 /> Database</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
    <NewTable parent={parent} open={naming} onOpenChange={setNaming} />
  </>
}
