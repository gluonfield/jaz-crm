import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { ArrowDownAZ, ChevronDown, Plus, Search, Tags } from 'lucide-react'
import { useState } from 'react'
import { Button, Header, inputClass } from '@/components/controls'
import { ConnectGoogle, EmptyState } from '@/components/empty-state'
import { ObjectIcon, RecordIcon } from '@/components/icons'
import { Picker } from '@/components/picker'
import { SelectField } from '@/components/select-field'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { formatDay } from '@/lib/format'
import { useDebounced, useListKeys } from '@/lib/hooks'
import { useAction, useObjects, useRecords } from '@/lib/queries'
import { useMail } from '@/lib/sync'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/o/$object')({
  validateSearch: (search: Record<string, unknown>): { category?: string; sort?: 'name' } => ({
    category: typeof search.category === 'string' ? search.category : undefined,
    sort: search.sort === 'name' ? 'name' : undefined,
  }),
  component: ObjectPage,
})

// arrivals says how synced objects' records arrive.
const arrivals: Record<string, string> = {
  people: 'People you write to or meet appear here automatically, along with anyone you keep in Triage.',
  companies: 'Companies appear here from the work email domains of the people you keep.',
}

function ObjectPage() {
  const { object: slug } = Route.useParams()
  const object = useObjects()?.find((o) => o.slug === slug)
  const categories = object?.attributes.find((a) => a.slug === 'categories' && a.type === 'select')
  const { category, sort } = Route.useSearch()
  const [search, setSearch] = useState('')
  const query = useDebounced(search.trim())
  const found = useRecords(slug, query, 100, category && categories ? { categories: category } : {}).data?.records
  const records = found && (sort === 'name' ? [...found].sort((a, b) => recordName(a).localeCompare(recordName(b))) : found)
  const navigate = useNavigate()
  const open = (index: number) => records && navigate({ to: '/r/$recordId', params: { recordId: records[index].id } })
  const [focus] = useListKeys(records?.length ?? 0, { Enter: open, o: open })
  const [creating, setCreating] = useState(false)
  if (!object) {
    return <Header>{slug}</Header>
  }
  const columns = object.attributes.filter((a) => a.slug !== 'name').sort((a, b) => Number(b.slug === 'categories') - Number(a.slug === 'categories'))
  const filter = (value: string) => void navigate({ to: '.', search: { category: value || undefined, sort }, replace: true })
  return (
    <>
      <Header>
        <ObjectIcon slug={slug} />
        {object.name}
        {records && <span className="font-normal tabular-nums text-ink-3">{records.length}</span>}
        <Button primary className="ml-auto h-10" onClick={() => setCreating(true)}>
          <Plus /> {slug === 'companies' ? 'New company' : 'New'}
        </Button>
      </Header>
      <div onKeyDown={(e) => e.stopPropagation()} className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border px-4 py-2.5">
        {categories && (
          <Picker
            trigger={<Button className="h-10 min-w-0 max-w-full"><Tags /><span className="max-w-56 truncate">{category || 'All categories'}</span><ChevronDown /></Button>}
            placeholder="Filter categories…"
            options={[{ value: '', label: 'All categories' }, ...(categories.options ?? []).map((value) => ({ value, label: value }))]}
            selected={[category ?? '']}
            onSelect={filter}
          />
        )}
        <Picker
          trigger={<Button className="h-10"><ArrowDownAZ />{sort === 'name' ? 'Name' : 'Recently added'}<ChevronDown /></Button>}
          placeholder="Sort records…"
          options={[{ value: '', label: 'Recently added' }, { value: 'name', label: 'Name' }]}
          selected={[sort ?? '']}
          onSelect={(value) => void navigate({ to: '.', search: { category, sort: value === 'name' ? 'name' : undefined }, replace: true })}
        />
        <label className="relative flex w-full min-w-0 items-center sm:ml-auto sm:w-64">
          <Search className="pointer-events-none absolute left-2 size-3.5 text-ink-3" />
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder={`Search ${object.name.toLowerCase()}`} aria-label={`Search ${object.name.toLowerCase()}`} className={cn(inputClass, 'h-10 w-full pl-7')} />
        </label>
      </div>
      {creating && <QuickCreate object={object} onDone={() => setCreating(false)} />}
      {records?.length === 0 && !creating ? (
        <Empty object={object} query={query} category={categories ? category : undefined} />
      ) : (
        <div className="scrollbar-quiet min-h-0 flex-1 overflow-auto">
          <table className="w-full border-collapse text-[13px]">
            <thead className="sticky top-0 z-10 bg-bg">
              <tr className="h-10 border-b border-border text-left text-[12px] text-ink-3">
                <th className="sticky left-0 z-20 min-w-56 bg-bg px-4 font-medium">{slug === 'companies' ? 'Company' : 'Name'}</th>
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
                  className={cn('group h-13 cursor-default border-b border-border/60 hover:bg-list-hover', focus === index && 'bg-list-hover')}
                >
                  <td className={cn('sticky left-0 z-10 max-w-72 bg-bg px-4 group-hover:bg-list-hover', focus === index && 'bg-list-hover')}>
                    <Link to="/r/$recordId" params={{ recordId: r.id }} onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()} className="flex min-h-10 items-center gap-2.5 font-medium text-ink">
                      <RecordIcon object={slug} name={recordName(r)} photo={r.photo} />
                      <span className="truncate">{recordName(r)}</span>
                    </Link>
                  </td>
                  {columns.map((a) => (
                    <td key={a.slug} className={cn('max-w-80 px-3 py-1 text-ink-2', a.type === 'select' ? 'min-w-64' : 'min-w-44')}>
                      {a.type === 'select' ? <div onClick={(e) => e.stopPropagation()}><SelectField record={r} attribute={a} /></div> : cell(r, a)}
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
  if (attribute.type === 'domain') {
    return <div className="flex flex-col gap-1">{values.map((domain) => <a key={domain} href={`https://${domain}`} target="_blank" rel="noreferrer" onClick={(e) => e.stopPropagation()} className="truncate text-primary hover:underline">{domain}</a>)}</div>
  }
  if (attribute.type === 'date') {
    return values.map(formatDay).join(', ')
  }
  if (attribute.type === 'checkbox') {
    return values[0] === 'true' ? 'Yes' : ''
  }
  if (attribute.type === 'number') {
    return values.map((v) => Number(v).toLocaleString('en')).join(', ')
  }
  return <span className="block truncate" title={values.join(', ')}>{values.join(', ') || '—'}</span>
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

function Empty({ object, query, category }: { object: CrmObject; query: string; category?: string }) {
  const mail = useMail()
  const plural = object.name.toLowerCase()
  if (category) {
    return <EmptyState title={`No ${plural} ${query ? `match “${query}” in` : 'in'} “${category}”`} icon={<Tags />} />
  }
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
