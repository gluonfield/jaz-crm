import { Button } from '@jaz/ui/button'
import { Plus } from 'lucide-react'
import type { CrmObject, CrmRecord } from '@/lib/types'
import { AddColumn, ColumnHeader, typeIcon } from './columns'
import { Field } from './fields'

// Properties shows a table record's columns under its title, as Notion shows
// a database page's properties: each edited in place, each label renaming
// or deleting its column, and Add a property adding one.
export function Properties({ record, object, objects }: { record: CrmRecord; object: CrmObject; objects: CrmObject[] }) {
  return (
    <div className="flex flex-col">
      <dl className="grid grid-cols-[minmax(0,160px)_minmax(0,1fr)] items-center gap-x-3 gap-y-0.5 text-[13px]">
        {object.attributes
          .filter((a) => a.slug !== 'name' && a.type !== 'markdown')
          .map((a) => (
            <div key={a.slug} className="contents">
              <dt className="min-w-0">
                <ColumnHeader object={object} attribute={a} icon={typeIcon(a)} className="font-normal text-ink-2 [&_svg]:size-4 [&_svg]:shrink-0 [&_svg]:text-ink-3" />
              </dt>
              <dd className="min-w-0">
                <Field record={record} attribute={a} />
              </dd>
            </div>
          ))}
      </dl>
      <AddColumn object={object} objects={objects}>
        <Button variant="ghost" className="-ml-2 mt-1 self-start">
          <Plus /> Add a property
        </Button>
      </AddColumn>
    </div>
  )
}
