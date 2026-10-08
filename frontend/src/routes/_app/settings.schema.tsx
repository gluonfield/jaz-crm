import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { Row, Section, inputClass } from '@/components/controls'
import { useAction, useTool } from '@/lib/queries'
import { cn } from '@/lib/utils'
import { Plus } from 'lucide-react'
import { NewAttribute } from '@/components/new-attribute'
import { ColumnHeader } from '@/components/columns'
import { slugify } from '@/lib/crm'
import type { CrmObject } from '@/lib/types'

export const Route = createFileRoute('/_app/settings/schema')({ component: SchemaPage })

function SchemaPage() {
  const objects = useTool<{ objects: CrmObject[] }>('list_objects', { include_archived: true }).data?.objects ?? []
  return (
    <Section title="Schema">
      {objects.map((o) => <ObjectRow key={o.slug} object={o} objects={objects} />)}
      <NewObject />
    </Section>
  )
}

function ObjectRow({ object, objects }: { object: CrmObject; objects: CrmObject[] }) {
  const [adding, setAdding] = useState(false)
  const edit = useAction<object>('edit_attribute')
  const archived = object.attributes.filter((attribute) => attribute.archived)
  return (
    <div className="border-b border-border/70 px-4 py-3 last:border-b-0">
      <div className="flex items-center gap-2 text-[13px]">
        <span className="font-medium text-ink">{object.name}</span>
        <span className="text-ink-3">{object.slug}</span>
        <Button variant="ghost" className="ml-auto" aria-label={`Add attribute to ${object.name}`} onClick={() => setAdding(!adding)}>
          <Plus /> Attribute
        </Button>
      </div>
      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[12.5px] text-ink-2">
        {object.attributes.filter((attribute) => !attribute.archived).map((a) => (
          <span key={a.slug} className="flex items-center gap-1">
            <ColumnHeader object={object} attribute={a} className="text-[12.5px] font-normal" />
            <span className="text-ink-3">{[a.type, a.target, a.multi && 'many', a.unique && 'unique'].filter(Boolean).join(' · ')}</span>
          </span>
        ))}
      </div>
      {archived.length > 0 && <details className="mt-2 text-[12.5px] text-ink-3">
        <summary className="cursor-pointer">Archived properties · {archived.length}</summary>
        <div className="mt-1 flex flex-col gap-1">
          {archived.map((attribute) => <div key={attribute.slug} className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span>{attribute.name}</span>
            <Button variant="ghost" disabled={edit.isPending} aria-label={`Restore ${attribute.name}`} onClick={() => edit.mutate({ object: object.slug, attribute: attribute.slug, action: 'restore' })}>Restore</Button>
          </div>)}
        </div>
      </details>}
      {adding && <NewAttribute object={object} objects={objects} onDone={() => setAdding(false)} className="mt-3" />}
    </div>
  )
}

function NewObject() {
  const [name, setName] = useState('')
  const create = useAction<object>('create_object')
  return (
    <Row>
      <form
        className="flex flex-1 gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          create.mutate({ slug: slugify(name), name }, { onSuccess: () => setName('') })
        }}
      >
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="New object, such as Deals" className={cn(inputClass, 'flex-1')} />
        <Button type="submit" disabled={!slugify(name)}>
          Create
        </Button>
      </form>
    </Row>
  )
}
