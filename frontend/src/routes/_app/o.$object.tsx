import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { Button, Header, inputClass } from '@/components/controls'
import { ConnectGoogle, EmptyState } from '@/components/empty-state'
import { ObjectIcon, RecordIcon } from '@/components/icons'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { formatDay } from '@/lib/format'
import { useDebounced, useListKeys } from '@/lib/hooks'
import { useAction, useObjects, useRecords } from '@/lib/queries'
import { useMail } from '@/lib/sync'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/o/$object')({ component: ObjectPage })

// arrivals says how synced objects' records arrive.
const arrivals: Record<string, string> = {
  people: 'People you write to or meet appear here automatically, along with anyone you keep in Triage.',
  companies: 'Companies appear here from the work email domains of the people you keep.',
}

function ObjectPage() {
  const { object: slug } = Route.useParams()
  const object = useObjects()?.find((o) => o.slug === slug)
  const [search, setSearch] = useState('')
  const query = useDebounced(search.trim())
  const records = useRecords(slug, query).data?.records
  const navigate = useNavigate()
  const open = (index: number) => records && navigate({ to: '/r/$recordId', params: { recordId: records[index].id } })
  const [focus] = useListKeys(records?.length ?? 0, { Enter: open, o: open })
  const [creating, setCreating] = useState(false)
  if (!object) {
    return <Header>{slug}</Header>
  }
  const columns = object.attributes.filter((a) => a.slug !== 'name')
  return (
    <>
      <Header>
        <ObjectIcon slug={slug} />
        {object.name}
        {records && <span className="font-normal tabular-nums text-ink-3">{records.length}</span>}
        <label className="relative ml-auto flex items-center">
          <Search className="pointer-events-none absolute left-2 size-3.5 text-ink-3" />
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search" aria-label={`Search ${object.name.toLowerCase()}`} className={cn(inputClass, 'w-56 pl-7')} />
        </label>
        <Button primary onClick={() => setCreating(true)}>
          <Plus /> New
        </Button>
      </Header>
      {creating && <QuickCreate object={object} onDone={() => setCreating(false)} />}
      {records?.length === 0 && !creating ? (
        <Empty object={object} query={query} />
      ) : (
        <div className="scrollbar-quiet min-h-0 flex-1 overflow-auto">
          <table className="w-full border-collapse text-[13px]">
            <thead className="sticky top-0 z-10 bg-bg">
              <tr className="h-8 border-b border-border text-left text-[12px] text-ink-3">
                <th className="px-4 font-medium">Name</th>
                {columns.map((a) => (
                  <th key={a.slug} className="whitespace-nowrap px-3 font-medium">
                    {a.name}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {records?.map((r, index) => (
                <tr
                  key={r.id}
                  data-row={index}
                  onClick={() => open(index)}
                  className={cn('h-9 cursor-default border-b border-border/60 hover:bg-list-hover', focus === index && 'bg-list-hover')}
                >
                  <td className="max-w-72 px-4">
                    <span className="flex items-center gap-2 truncate font-medium text-ink">
                      <RecordIcon object={slug} name={recordName(r)} photo={r.photo} />
                      {recordName(r)}
                    </span>
                  </td>
                  {columns.map((a) => (
                    <td key={a.slug} className="max-w-60 truncate px-3 text-ink-2">
                      {cell(r, a)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}

function cell(record: CrmRecord, attribute: Attribute) {
  const values = valuesOf(record, attribute.slug).map(valueText)
  if (attribute.type === 'date') {
    return values.map(formatDay).join(', ')
  }
  if (attribute.type === 'checkbox') {
    return values[0] === 'true' ? 'Yes' : ''
  }
  if (attribute.type === 'number') {
    return values.map((v) => Number(v).toLocaleString('en')).join(', ')
  }
  return values.join(', ')
}

// QuickCreate adds a record by name and its identifying value, such as an
// email or domain, which also finds the record if it already exists.
function QuickCreate({ object, onDone }: { object: CrmObject; onDone: () => void }) {
  const identity = object.attributes.find((a) => a.unique)
  const [name, setName] = useState('')
  const [key, setKey] = useState('')
  const navigate = useNavigate()
  const upsert = useAction<object, { record: CrmRecord }>('upsert_record')
  const submit = () => {
    const values: Record<string, string> = {}
    if (name.trim()) {
      values.name = name.trim()
    }
    if (identity && key.trim()) {
      values[identity.slug] = key.trim()
    }
    upsert.mutate({ object: object.slug, values }, { onSuccess: (out) => navigate({ to: '/r/$recordId', params: { recordId: out.record.id } }) })
  }
  return (
    <form
      className="flex items-center gap-2 border-b border-border bg-panel px-4 py-2"
      onSubmit={(e) => {
        e.preventDefault()
        submit()
      }}
      onKeyDown={(e) => e.key === 'Escape' && onDone()}
    >
      <input autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder="Name" className={cn(inputClass, 'w-56')} />
      {identity && <input value={key} onChange={(e) => setKey(e.target.value)} placeholder={identity.name} className={cn(inputClass, 'w-64')} />}
      <Button primary type="submit" disabled={!name.trim() && !key.trim()}>
        Create
      </Button>
      <Button onClick={onDone}>Cancel</Button>
    </form>
  )
}

function Empty({ object, query }: { object: CrmObject; query: string }) {
  const mail = useMail()
  const plural = object.name.toLowerCase()
  if (query) {
    return <EmptyState title={`No ${plural} match “${query}”`} icon={<Search />} />
  }
  const synced = object.slug in arrivals
  return (
    <EmptyState title={`No ${plural} yet`} icon={<ObjectIcon slug={object.slug} />} action={synced && mail === 'none' && <ConnectGoogle />}>
      {arrivals[object.slug] ?? 'Records you add with New, or that an agent creates, appear here.'}
    </EmptyState>
  )
}
