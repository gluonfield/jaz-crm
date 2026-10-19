import { Link } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { Button } from '@jaz/ui/button'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { pageIcon } from '@/lib/pages'
import { formatDateTime, formatDay, formatNumber, recentOrDate } from '@/lib/format'
import { useColumnWidths } from '@/lib/use-column-widths'
import { useTableOrder } from '@/lib/use-table-order'
import type { Attribute, CrmObject, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'
import { AddColumn, ColumnHeader } from './columns'
import { CompanyPeople } from './company-people'
import { DateLabel, dateMetadata } from './date-field'
import { valueLinks } from './external-link'
import { Field } from './fields'
import { RecordIcon } from './icons'
import { RecordMenu } from './record-menu'
import { SelectField } from './select-field'
import { Stage } from './stage'
import { UpdatedAt } from './updated-at'

type TableColumn = { key: string; name: string; width: number; attribute?: Attribute }

export function RecordTable({ object, objects, records, focus, openRecord }: {
  object: CrmObject
  objects: CrmObject[]
  records: CrmRecord[]
  focus: number
  openRecord: (index: number) => void
}) {
  const slug = object.slug
  const own = !object.standard
  const { width, resize } = useColumnWidths(slug)
  const attributes = object.attributes.filter((a) => a.slug !== 'name' && a.type !== 'markdown' && !(slug === 'follow_ups' && dateMetadata(a.slug)))
    .sort((a, b) => own ? 0 : Number(b.type === 'select') - Number(a.type === 'select'))
  const defaults: TableColumn[] = [
    ...(slug === 'companies' ? [{ key: '$people', name: 'People', width: 240 }] : []),
    ...(['companies', 'people'].includes(slug) ? [{ key: '$last_contact', name: 'Last Contact', width: 140 }] : []),
    { key: '$updated_at', name: 'Last updated', width: 140 },
    ...attributes.map((attribute) => ({ key: attribute.slug, name: attribute.name, width: defaultWidth(attribute), attribute })),
  ]
  const ordering = useTableOrder(object, defaults.map((column) => column.key))
  const columns = ordering.order.map((key) => defaults.find((column) => column.key === key)!)
  return (
    <table className="w-full table-fixed border-collapse text-[13px] [&_[data-hint]]:opacity-0 [&_[data-hint]:focus-visible]:opacity-100 [&_[data-hint][data-state=open]]:opacity-100 [&_tr:hover_[data-hint]]:opacity-100 [&_tr:not(:hover)_input:not(:focus)::placeholder]:text-transparent pointer-coarse:[&_[data-hint]]:opacity-100">
      <colgroup>
        <col style={{ width: width('name', 280) }} />
        {columns.map((column) => <col key={column.key} style={{ width: width(column.key, column.width) }} />)}
        {own && <col style={{ width: 48 }} />}
        <col />
      </colgroup>
      <thead className="sticky top-0 z-10 bg-bg">
        <tr className="h-9 border-b border-border text-left text-[12px] text-ink-3">
          <th data-column="name" className="sticky left-0 z-20 bg-bg px-4 font-medium">
            {own ? <ColumnHeader object={object} attribute={object.attributes.find((a) => a.slug === 'name')!} /> : slug === 'companies' ? 'Company' : 'Name'}
            <Resizer onPointerDown={resize('name', 280)} />
          </th>
          {columns.map((column, index) => (
            <th key={column.key} data-column={column.key} title="Drag to reorder" {...ordering.header(column.key)} className={cn('group/column relative select-none px-3 font-medium', !ordering.pending && 'cursor-grab', ordering.dragging === column.key && 'opacity-50')}>
              <div className="flex items-center gap-1">
                <ColumnHeader object={object} attribute={own ? column.attribute : undefined} name={column.name} reorder={{
                  left: !ordering.pending && index > 0 ? () => ordering.move(column.key, index - 1) : undefined,
                  right: !ordering.pending && index < columns.length - 1 ? () => ordering.move(column.key, index + 1) : undefined,
                }} />
              </div>
              {ordering.target?.key === column.key && ordering.dragging !== column.key && <span data-column-drop aria-hidden className={cn('pointer-events-none absolute inset-y-0 z-30 w-0.5 bg-primary', ordering.target.after ? 'right-0' : 'left-0')} />}
              <Resizer onPointerDown={resize(column.key, column.width)} />
            </th>
          ))}
          {own && (
            <th className="px-1">
              <AddColumn object={object} objects={objects}>
                <Button variant="ghost" size="icon-sm" aria-label="Add column">
                  <Plus />
                </Button>
              </AddColumn>
            </th>
          )}
          <th />
        </tr>
      </thead>
      <tbody>
        {records.map((r, index) => (
          <RecordMenu key={r.id} object={object} record={r}>
            <tr
              data-row={index}
              data-flip={r.id}
              onClick={() => openRecord(index)}
              className={cn('group h-11 cursor-default border-b border-border/60 hover:bg-list-hover data-[state=open]:bg-list-hover', focus === index && 'bg-list-hover')}
            >
              <td data-column="name" className={cn('sticky left-0 z-10 bg-bg px-4 py-2 group-hover:bg-list-hover group-data-[state=open]:bg-list-hover', focus === index && 'bg-list-hover')}>
                <Link to="/r/$recordId" params={{ recordId: r.id }} onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()} className="flex min-w-0 items-center gap-2.5 font-medium leading-[1.35] text-ink">
                  <RecordIcon object={slug} name={recordName(r)} photo={r.photo} icon={pageIcon(r)} />
                  <span title={recordName(r)} className="line-clamp-2 min-w-0">{recordName(r)}</span>
                </Link>
              </td>
              {columns.map((column) => (
                <td key={column.key} data-column={column.key} className="overflow-hidden px-3 py-1 text-ink-2">
                  <TableCell record={r} column={column} editable={own} />
                </td>
              ))}
              {own && <td />}
              <td />
            </tr>
          </RecordMenu>
        ))}
      </tbody>
    </table>
  )
}

function TableCell({ record, column, editable }: { record: CrmRecord; column: TableColumn; editable: boolean }) {
  const attribute = column.attribute
  if (attribute) {
    if (editable || attribute.type === 'select') {
      return <div onClick={(e) => e.stopPropagation()}>
        {editable ? <Field record={record} attribute={attribute} inline /> : <SelectField record={record} attribute={attribute} inline />}
      </div>
    }
    return cell(record, attribute)
  }
  if (column.key === '$people') {
    return <CompanyPeople company={record} />
  }
  return <span className="text-[12px] text-ink-3">
    {column.key === '$updated_at' ? <UpdatedAt at={record.updated_at} /> : record.activity?.last_at && <time dateTime={record.activity.last_at} title={formatDateTime(record.activity.last_at)} className="whitespace-nowrap tabular-nums">{recentOrDate(record.activity.last_at)}</time>}
  </span>
}

// defaultWidth fits a column's usual content until someone drags it wider.
const defaultWidth = (attribute: Attribute) => (attribute.multi || attribute.type === 'reference' ? 280 : attribute.type === 'select' || attribute.type === 'status' ? 200 : 180)

// Resizer is the edge of a table header that drags its column's width.
function Resizer({ onPointerDown }: { onPointerDown: (e: React.PointerEvent) => void }) {
  return <span aria-hidden onPointerDown={onPointerDown} onClick={(e) => e.stopPropagation()} className="absolute inset-y-0 right-0 w-1.5 cursor-col-resize touch-none transition-colors hover:bg-primary/40" />
}

function cell(record: CrmRecord, attribute: Attribute) {
  const values = valuesOf(record, attribute.slug).map(valueText)
  if (attribute.type === 'status' && values[0]) {
    return <Stage stage={values[0]} />
  }
  const ValueLink = valueLinks[attribute.type]
  if (ValueLink && values[0]) {
    return (
      <span className="flex min-w-0 items-center gap-1.5">
        <ValueLink value={values[0]} className="truncate" />
        {values.length > 1 && <span className="shrink-0 text-[12px] text-ink-3">+{values.length - 1}</span>}
      </span>
    )
  }
  if (attribute.type === 'datetime') {
    return values.map((value) => <DateLabel key={value} value={value} suggested={attribute.slug === 'action_date' && record.values.action_date_basis === 'Suggested'} />)
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
