import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { ArrowDownAZ, Kanban, ListChecks, Plus, Search, Table2 } from 'lucide-react'
import { type ReactNode, useEffect, useRef, useState } from 'react'
import { Board } from '@/components/board'
import { CompanyPeople } from '@/components/company-people'
import { editDraft, FollowUpQueue } from '@/components/follow-ups'
import { Stage } from '@/components/stage'
import { Button } from '@jaz/ui/button'
import { Header } from '@/components/controls'
import { AddColumn, ColumnHeader } from '@/components/columns'
import { CreateRecord } from '@/components/create-record'
import { Field } from '@/components/fields'
import { ExternalLink } from '@/components/external-link'
import { ConnectGoogle, EmptyState } from '@/components/empty-state'
import { ObjectIcon, RecordIcon } from '@/components/icons'
import { Picker } from '@/components/picker'
import { RecordFilters } from '@/components/record-filters'
import { RecordMenu } from '@/components/record-menu'
import { SelectField } from '@/components/select-field'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { formatDay, formatNumber } from '@/lib/format'
import { useDebounced, useFlip, useInView, useListKeys } from '@/lib/hooks'
import { useObjects, useRecordPages } from '@/lib/queries'
import { useColumnWidths } from '@/lib/use-column-widths'
import { validateRecordSearch } from '@/lib/record-search'
import { statusOf } from '@/lib/stages'
import { useMail } from '@/lib/sync'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/o/$object')({
  validateSearch: validateRecordSearch,
  component: ObjectPage,
})

// arrivals says how synced objects' records arrive.
const arrivals: Record<string, string> = {
  people: 'People you write to or meet appear here automatically, along with anyone you keep in Triage.',
  companies: 'Companies appear here from the work email domains of the people you keep.',
  follow_ups: 'Next steps appear here as conversations with the people you keep create them.',
}

// open is the queue's default view: the follow-ups still to act on.
const open = [{ attribute: 'status', operator: 'is' as const, value: 'Open' }]

function ObjectPage() {
  const { object: slug } = Route.useParams()
  const objects = useObjects()
  const object = objects?.find((o) => o.slug === slug)
  const search = Route.useSearch()
  const { sort, view, q = '', filters = [], saved, limit = 100 } = search
  const query = useDebounced(q.trim())
  const queue = slug === 'follow_ups' && view !== 'table'
  const result = useRecordPages(slug, query, queue && filters.length === 0 ? open : filters, slug === 'companies' ? [{ object: 'people', attribute: 'company', limit: 4 }] : undefined, queue ? 'review_on' : sort, limit)
  const { records, total, hasNextPage, isFetchingNextPage, fetchNextPage } = result
  const { width, resize } = useColumnWidths(slug)
  const status = object && statusOf(object.attributes)
  // The workspace's own tables open as tables, the CRM's pipelines as boards.
  const board = slug !== 'follow_ups' && (view ?? (object?.standard ? 'board' : 'table')) === 'board' ? status : undefined
  // A board or queue shows every record; a table loads more as it scrolls.
  const everything = !!board || queue
  useEffect(() => {
    if (everything && hasNextPage && !isFetchingNextPage) {
      void fetchNextPage()
    }
  }, [everything, hasNextPage, isFetchingNextPage, fetchNextPage])
  const end = useRef<HTMLDivElement>(null)
  useInView(end, !everything && !!hasNextPage && !isFetchingNextPage, fetchNextPage)
  const navigate = useNavigate()
  const openRecord = (index: number) => records && navigate({ to: '/r/$recordId', params: { recordId: records[index].id } })
  const [focus, setFocus] = useListKeys(records?.length ?? 0, queue ? { Enter: editDraft, Escape: () => setFocus(-1) } : { Enter: openRecord, o: openRecord })
  const [creating, setCreating] = useState(false)
  const rows = useRef<HTMLTableSectionElement>(null)
  useFlip(rows)
  if (!object) {
    return <Header>{slug}</Header>
  }
  const own = !object.standard
  // The CRM's objects show their choices first; the workspace's own tables keep the order columns were added in.
  const columns = object.attributes.filter((a) => a.slug !== 'name' && a.type !== 'markdown').sort((a, b) => (own ? 0 : Number(b.type === 'select') - Number(a.type === 'select')))
  return (
    <>
      <Header>
        <ObjectIcon slug={slug} />
        <span className="whitespace-nowrap">{object.name}</span>
        {total !== undefined && <span className="font-normal tabular-nums text-ink-3">{total}</span>}
        {status && (
          <div role="group" aria-label="View" className="ml-2 flex h-7 items-center rounded-full bg-list-hover p-0.5">
            <ViewButton active={!!board || queue} label={slug === 'follow_ups' ? 'Queue' : 'Board'} onClick={() => void navigate({ to: '.', search: { ...search, view: own ? 'board' : undefined }, replace: true })}>
              {slug === 'follow_ups' ? <ListChecks /> : <Kanban />}
            </ViewButton>
            <ViewButton active={!board && !queue} label="Table" onClick={() => void navigate({ to: '.', search: { ...search, view: own ? undefined : 'table' }, replace: true })}>
              <Table2 />
            </ViewButton>
          </div>
        )}
        <div className="ml-auto flex min-w-0 items-center gap-1 font-normal">
          <RecordFilters key={slug} object={object} filters={filters} query={q} selected={saved}
            onChange={(filters) => void navigate({ to: '.', search: { ...search, filters }, replace: true })}
            onApply={(filter) => void navigate({ to: '.', search: { ...search, filters: filter?.filters ?? [], q: filter?.query, saved: filter?.id }, replace: true })}
          />
          <Picker
            trigger={
              <Button variant="ghost" aria-label="Sort records">
                <ArrowDownAZ />
                <span className="hidden xl:inline">{sort === 'name' ? 'Name' : queue ? 'Review date' : 'Recently added'}</span>
              </Button>
            }
            placeholder="Sort by…"
            options={[{ value: '', label: queue ? 'Review date' : 'Recently added' }, { value: 'name', label: 'Name' }]}
            selected={[sort ?? '']}
            onSelect={(value) => void navigate({ to: '.', search: { ...search, sort: value === 'name' ? 'name' : undefined }, replace: true })}
          />
          <label className="group flex h-7 min-w-0 items-center gap-1.5 rounded-[var(--radius-control)] px-2 text-ink-3 transition-colors focus-within:bg-list-hover hover:bg-list-hover">
            <Search className="size-3.5 shrink-0" />
            <input
              type="search"
              autoComplete="off"
              value={q}
              onChange={(e) => void navigate({ to: '.', search: { ...search, q: e.target.value || undefined }, replace: true })}
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
      {result.isError ? <EmptyState title={result.error.message} icon={<Search />} /> : board ? (
        records && <Board object={object} status={board} records={records} />
      ) : records?.length === 0 && (!own || query || filters.length > 0) ? (
        <Empty object={object} query={query} filtered={filters.length > 0} />
      ) : queue ? (
        records && <FollowUpQueue records={records} focus={focus} onFocus={setFocus} />
      ) : (
        <div className="scrollbar-quiet min-h-0 flex-1 overflow-auto">
          <table className="w-full table-fixed border-collapse text-[13px]">
            <colgroup>
              <col style={{ width: width('name', 280) }} />
              {slug === 'companies' && <col style={{ width: width('people', 240) }} />}
              {columns.map((a) => (
                <col key={a.slug} style={{ width: width(a.slug, defaultWidth(a)) }} />
              ))}
              {own && <col style={{ width: 48 }} />}
              <col />
            </colgroup>
            <thead className="sticky top-0 z-10 bg-bg">
              <tr className="h-9 border-b border-border text-left text-[12px] text-ink-3">
                <th className="sticky left-0 z-20 bg-bg px-4 font-medium">
                  {own ? <ColumnHeader object={object} attribute={object.attributes.find((a) => a.slug === 'name')!} /> : slug === 'companies' ? 'Company' : 'Name'}
                  <Resizer onPointerDown={resize('name', 280)} />
                </th>
                {slug === 'companies' && (
                  <th className="relative px-3 font-medium">
                    People
                    <Resizer onPointerDown={resize('people', 240)} />
                  </th>
                )}
                {columns.map((a) => (
                  <th key={a.slug} className="relative px-3 font-medium">
                    {own ? <ColumnHeader object={object} attribute={a} /> : <span className="block truncate">{a.name}</span>}
                    <Resizer onPointerDown={resize(a.slug, defaultWidth(a))} />
                  </th>
                ))}
                {own && (
                  <th className="px-1">
                    <AddColumn object={object} objects={objects ?? []}>
                      <Button variant="ghost" size="icon-sm" aria-label="Add column">
                        <Plus />
                      </Button>
                    </AddColumn>
                  </th>
                )}
                <th />
              </tr>
            </thead>
            <tbody ref={rows}>
              {records?.map((r, index) => (
                <RecordMenu key={r.id} object={object} record={r}>
                  <tr
                    data-row={index}
                    data-flip={r.id}
                    onClick={() => openRecord(index)}
                    className={cn('group h-10 cursor-default border-b border-border/50 hover:bg-list-hover data-[state=open]:bg-list-hover', focus === index && 'bg-list-hover')}
                  >
                    <td className={cn('sticky left-0 z-10 bg-bg px-4 group-hover:bg-list-hover group-data-[state=open]:bg-list-hover', focus === index && 'bg-list-hover')}>
                      <Link to="/r/$recordId" params={{ recordId: r.id }} onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()} className="flex min-w-0 items-center gap-2.5 font-medium text-ink">
                        <RecordIcon object={slug} name={recordName(r)} photo={r.photo} />
                        <span className="min-w-0 truncate">{recordName(r)}</span>
                      </Link>
                    </td>
                    {slug === 'companies' && <td className="overflow-hidden px-3 text-ink-2"><CompanyPeople company={r} /></td>}
                    {columns.map((a) => (
                      <td key={a.slug} className="overflow-hidden px-3 py-1 text-ink-2">
                        {own ? (
                          <div onClick={(e) => e.stopPropagation()}>
                            <Field record={r} attribute={a} limit={5} />
                          </div>
                        ) : a.type === 'select' ? (
                          <div onClick={(e) => e.stopPropagation()}>
                            <SelectField record={r} attribute={a} />
                          </div>
                        ) : (
                          cell(r, a)
                        )}
                      </td>
                    ))}
                    {own && <td />}
                    <td />
                  </tr>
                </RecordMenu>
              ))}
            </tbody>
          </table>
          <div ref={end} aria-hidden className="h-px" />
        </div>
      )}
    </>
  )
}

// defaultWidth fits a column's usual content until someone drags it wider.
const defaultWidth = (attribute: Attribute) => (attribute.multi || attribute.type === 'reference' ? 280 : attribute.type === 'select' || attribute.type === 'status' ? 200 : 180)

// Resizer is the edge of a table header that drags its column's width.
function Resizer({ onPointerDown }: { onPointerDown: (e: React.PointerEvent) => void }) {
  return <span aria-hidden onPointerDown={onPointerDown} onClick={(e) => e.stopPropagation()} className="absolute inset-y-0 right-0 w-1.5 cursor-col-resize touch-none transition-colors hover:bg-primary/40" />
}

function ViewButton({ active, label, onClick, children }: { active: boolean; label: string; onClick: () => void; children: ReactNode }) {
  return (
    <Button variant="ghost" size="icon-sm" aria-label={label} aria-pressed={active} title={label} onClick={onClick} className={active ? 'bg-raised text-ink' : undefined}>
      {children}
    </Button>
  )
}

function cell(record: CrmRecord, attribute: Attribute) {
  const values = valuesOf(record, attribute.slug).map(valueText)
  if (attribute.type === 'status' && values[0]) {
    return <Stage stage={values[0]} />
  }
  if (attribute.type === 'domain' || attribute.type === 'url') {
    return <div className="flex flex-col gap-1">{values.map((value) => <ExternalLink key={value} href={attribute.type === 'domain' ? `https://${value}` : value} className="block truncate">{value}</ExternalLink>)}</div>
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

function Empty({ object, query, filtered }: { object: CrmObject; query: string; filtered: boolean }) {
  const mail = useMail()
  const plural = object.name.toLowerCase()
  if (query) {
    return <EmptyState title={`No ${plural} match “${query}”`} icon={<Search />} />
  }
  if (filtered) {
    return <EmptyState title={`No ${plural} match this view`} icon={<Search />} />
  }
  const synced = object.slug in arrivals
  return (
    <EmptyState title={`No ${plural} yet`} icon={<ObjectIcon slug={object.slug} />} action={synced && mail === 'none' && <ConnectGoogle />}>
      {arrivals[object.slug] ?? 'Records you add with New, or that an agent creates, appear here.'}
    </EmptyState>
  )
}
