import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { ArrowDownAZ, Kanban, Plus, Search, Table2, Tags } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { Board } from '@/components/board'
import { Stage } from '@/components/stage'
import { Button, Header } from '@/components/controls'
import { CreateRecord } from '@/components/create-record'
import { ConnectGoogle, EmptyState } from '@/components/empty-state'
import { ObjectIcon, RecordIcon } from '@/components/icons'
import { Picker } from '@/components/picker'
import { SelectField } from '@/components/select-field'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { formatDay, formatNumber } from '@/lib/format'
import { useDebounced, useListKeys } from '@/lib/hooks'
import { useObjects, useRecords } from '@/lib/queries'
import { statusOf } from '@/lib/stages'
import { useMail } from '@/lib/sync'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/o/$object')({
  validateSearch: (search: Record<string, unknown>): { category?: string; sort?: 'name'; view?: 'table' } => ({
    view: search.view === 'table' ? 'table' : undefined,
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
  const { category, sort, view } = Route.useSearch()
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
  const status = statusOf(object.attributes)
  const board = view === 'table' ? undefined : status
  const filter = (value: string) => void navigate({ to: '.', search: { category: value || undefined, sort, view }, replace: true })
  return (
    <>
      <Header>
        <ObjectIcon slug={slug} />
        {object.name}
        {records && <span className="font-normal tabular-nums text-ink-3">{records.length}</span>}
        {status && (
          <div role="group" aria-label="View" className="ml-2 flex h-7 items-center rounded-full bg-list-hover p-0.5">
            <ViewButton active={!!board} label="Board" onClick={() => void navigate({ to: '.', search: { category, sort }, replace: true })}>
              <Kanban />
            </ViewButton>
            <ViewButton active={!board} label="Table" onClick={() => void navigate({ to: '.', search: { category, sort, view: 'table' }, replace: true })}>
              <Table2 />
            </ViewButton>
          </div>
        )}
        <div className="ml-auto flex min-w-0 items-center gap-1 font-normal">
          {categories && (
            <Picker
              trigger={
                <Button ghost className="min-w-0">
                  <Tags />
                  <span className="max-w-40 truncate">{category || 'All categories'}</span>
                </Button>
              }
              placeholder="Filter categories…"
              options={[{ value: '', label: 'All categories' }, ...(categories.options ?? []).map((value) => ({ value, label: value }))]}
              selected={[category ?? '']}
              onSelect={filter}
            />
          )}
          <Picker
            trigger={
              <Button ghost>
                <ArrowDownAZ />
                {sort === 'name' ? 'Name' : 'Recently added'}
              </Button>
            }
            placeholder="Sort by…"
            options={[{ value: '', label: 'Recently added' }, { value: 'name', label: 'Name' }]}
            selected={[sort ?? '']}
            onSelect={(value) => void navigate({ to: '.', search: { category, sort: value === 'name' ? 'name' : undefined, view }, replace: true })}
          />
          <label className="group flex h-7 min-w-0 items-center gap-1.5 rounded-[var(--radius-control)] px-2 text-ink-3 transition-colors focus-within:bg-list-hover hover:bg-list-hover">
            <Search className="size-3.5 shrink-0" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyDown={(e) => e.stopPropagation()}
              placeholder="Search"
              aria-label={`Search ${object.name.toLowerCase()}`}
              className="w-20 min-w-0 bg-transparent text-[12.5px] text-ink outline-none transition-[width] duration-150 placeholder:text-ink-3 focus:w-40"
            />
          </label>
          <Button className="ml-1" onClick={() => setCreating(true)}>
            <Plus /> New
          </Button>
        </div>
      </Header>
      <CreateRecord object={object} open={creating} onOpenChange={setCreating} openCreated />
      {board ? (
        records && <Board object={object} status={board} records={records} />
      ) : records?.length === 0 ? (
        <Empty object={object} query={query} category={categories ? category : undefined} />
      ) : (
        <div className="scrollbar-quiet min-h-0 flex-1 overflow-auto">
          <table className="w-full border-collapse text-[13px]">
            <thead className="sticky top-0 z-10 bg-bg">
              <tr className="h-9 border-b border-border text-left text-[12px] text-ink-3">
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
                  className={cn('group h-10 cursor-default border-b border-border/50 hover:bg-list-hover', focus === index && 'bg-list-hover')}
                >
                  <td className={cn('sticky left-0 z-10 max-w-72 bg-bg px-4 group-hover:bg-list-hover', focus === index && 'bg-list-hover')}>
                    <Link to="/r/$recordId" params={{ recordId: r.id }} onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()} className="flex items-center gap-2.5 font-medium text-ink">
                      <RecordIcon object={slug} name={recordName(r)} photo={r.photo} />
                      <span className="truncate">{recordName(r)}</span>
                    </Link>
                  </td>
                  {columns.map((a) => (
                    <td key={a.slug} className={cn('max-w-80 px-3 text-ink-2', a.type === 'select' ? 'min-w-56' : 'min-w-40')}>
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

function ViewButton({ active, label, onClick, children }: { active: boolean; label: string; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={active}
      title={label}
      onClick={onClick}
      className={cn(
        'flex h-full items-center rounded-full px-2 text-ink-3 outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring [&_svg]:size-3.5',
        active ? 'bg-raised text-ink shadow-xs' : 'hover:text-ink',
      )}
    >
      {children}
    </button>
  )
}

function cell(record: CrmRecord, attribute: Attribute) {
  const values = valuesOf(record, attribute.slug).map(valueText)
  if (attribute.type === 'status' && values[0]) {
    return <Stage stage={values[0]} />
  }
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
    return values.map((v) => formatNumber(v, attribute.slug)).join(', ')
  }
  return <span className="block truncate" title={values.join(', ')}>{values.join(', ')}</span>
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
