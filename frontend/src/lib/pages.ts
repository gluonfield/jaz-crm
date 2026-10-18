import { type DragEvent, useMemo, useState } from 'react'
import { recordName, valuesOf } from './crm'
import { useAction, useObjects, useTool } from './queries'
import type { CrmObject, CrmRecord, Ref } from './types'

// A page's order sorts it among its siblings: its stored position, else when
// it was made. parent is where it sits in the tree.
export type Page = { id: string; name: string; parent?: string; icon?: string; order: number }

export const pageIcon = (record: CrmRecord) => record.object === 'pages' && typeof record.values.icon === 'string' ? record.values.icon : undefined

// Pages is every page, and its sub-pages by parent ('' for the top level) in
// order.
export type Pages = { byId: Map<string, Page>; children: Map<string, Page[]>; tables: Map<string, CrmObject[]> }

export function usePages(): Pages | undefined {
  const records = useTool<{ records: CrmRecord[] }>('search_records', { object: 'pages', limit: 500 }).data?.records
  const objects = useObjects()
  return useMemo(() => records && objects && pagesOf(records, objects), [records, objects])
}

export function pagesOf(records: CrmRecord[], objects: CrmObject[]): Pages {
  const byId = new Map<string, Page>()
  for (const r of [...records].sort((a, b) => a.created_at.localeCompare(b.created_at))) {
    const position = valuesOf(r, 'position')[0]
    byId.set(r.id, { id: r.id, name: recordName(r), parent: (valuesOf(r, 'parent')[0] as Ref | undefined)?.id, icon: pageIcon(r), order: typeof position === 'string' ? Number(position) : Date.parse(r.created_at) })
  }
  // A page whose parent is unknown, or which no top-level page reaches, sits
  // at the top level.
  const top = (page: Page) => !page.parent || !byId.has(page.parent) || path({ byId }, page.id).length === 0
  const sorted = [...byId.values()].sort((a, b) => a.order - b.order)
  const parents = sorted.map((page) => (top(page) ? '' : page.parent!))
  const children = new Map<string, Page[]>()
  sorted.forEach((page, i) => {
    page.parent = parents[i] || undefined
    children.set(parents[i], [...(children.get(parents[i]) ?? []), page])
  })
  const tables = new Map<string, CrmObject[]>()
  for (const table of objects.filter((object) => !object.standard)) {
    const parent = table.parent && byId.has(table.parent) ? table.parent : ''
    tables.set(parent, [...(tables.get(parent) ?? []), table])
  }
  return { byId, children, tables }
}

// path lists a page's ancestors and the page from the top, or nothing when
// its parents loop.
export function path(pages: Pick<Pages, 'byId'>, id: string): Page[] {
  const out: Page[] = []
  for (let page = pages.byId.get(id); page; page = page.parent ? pages.byId.get(page.parent) : undefined) {
    if (out.includes(page)) {
      return []
    }
    out.unshift(page)
  }
  return out
}

// within reports whether a page is the given one or inside it.
export const within = (pages: Pages, id: string, ancestor: string) => path(pages, id).some((p) => p.id === ancestor)

// Drop is where a dragged page or table lands: inside a page ('' for the top
// level), or before or after one among its siblings.
export type Place = 'inside' | 'before' | 'after'
export type Drop = { id: string; place: Place }

// slot is where a page dropped there sits: its parent, and an order between
// its new neighbours, after the last page when it goes inside.
export function slot(pages: Pages, page: string, drop: Drop) {
  const parent = drop.place === 'inside' ? drop.id : (pages.byId.get(drop.id)?.parent ?? '')
  const siblings = (pages.children.get(parent) ?? []).filter((p) => p.id !== page)
  const at = drop.place === 'inside' ? siblings.length : siblings.findIndex((p) => p.id === drop.id) + (drop.place === 'after' ? 1 : 0)
  const [before, after] = [siblings[at - 1], siblings[at]]
  const order = before && after ? (before.order + after.order) / 2 : before ? before.order + 1000 : after ? after.order - 1000 : 0
  return { parent, order }
}

export function useDataDrag(pages: Pages | undefined, moved: (parent: string) => void) {
  const write = useAction<object>('upsert_record')
  const edit = useAction<object>('edit_object')
  const [source, setSource] = useState<Page | CrmObject>()
  const [target, setTarget] = useState<Drop>()
  const type = 'application/x-jaz-crm-data'
  const pending = write.isPending || edit.isPending
  const key = (item: Page | CrmObject) => 'slug' in item ? `database:${item.slug}` : `page:${item.id}`
  const reset = () => {
    setSource(undefined)
    setTarget(undefined)
  }
  // A table only moves inside a page; a page also moves beside one, where the
  // row offers it, by its top or bottom edge.
  const dropAt = (event: DragEvent<HTMLElement>, id: string, edges: Place[]): Drop => {
    const { top, height } = event.currentTarget.getBoundingClientRect()
    const y = (event.clientY - top) / height
    const edge = !source || 'slug' in source ? undefined : y < 0.3 ? 'before' : y > 0.7 ? 'after' : undefined
    return { id, place: edge && edges.includes(edge) ? edge : 'inside' }
  }
  const accepts = ({ id, place }: Drop) => {
    if (!pages || !source || pending || (id !== '' && !pages.byId.has(id))) {
      return false
    }
    const parent = place === 'inside' ? id : (pages.byId.get(id)?.parent ?? '')
    if (place === 'inside' && parent === (source.parent ?? '')) {
      return false
    }
    return 'slug' in source || (id !== source.id && !within(pages, parent, source.id))
  }
  return {
    dragging: !!source,
    target,
    from: (item: Page | CrmObject) => ({
      draggable: !pending,
      'data-dragging': !!source && key(source) === key(item),
      onDragStart: (event: DragEvent<HTMLElement>) => {
        if (pending) {
          event.preventDefault()
          return
        }
        event.dataTransfer.clearData()
        event.dataTransfer.setData(type, key(item))
        event.dataTransfer.effectAllowed = 'move'
        setSource(item)
      },
      onDragEnd: reset,
    }),
    // onto makes a row a drop target for the page id ('' for the top level),
    // offering the edges a page can also land beside it by.
    onto: (id: string, edges: Place[] = []) => ({
      onDragOver: (event: DragEvent<HTMLElement>) => {
        if (!event.dataTransfer.types.includes(type)) {
          return
        }
        event.stopPropagation()
        const drop = dropAt(event, id, edges)
        const allowed = accepts(drop)
        event.dataTransfer.dropEffect = allowed ? 'move' : 'none'
        setTarget((current) => !allowed ? undefined : current?.id === id && current.place === drop.place ? current : drop)
        if (allowed) {
          event.preventDefault()
        }
      },
      onDragLeave: (event: DragEvent<HTMLElement>) => {
        if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) {
          setTarget((current) => current?.id === id ? undefined : current)
        }
      },
      onDrop: (event: DragEvent<HTMLElement>) => {
        if (!event.dataTransfer.types.includes(type)) {
          return
        }
        event.preventDefault()
        event.stopPropagation()
        const drop = dropAt(event, id, edges)
        if (pages && source && event.dataTransfer.getData(type) === key(source) && accepts(drop)) {
          if ('slug' in source) {
            edit.mutate({ object: source.slug, action: 'move', parent: id }, { onSuccess: () => moved(id) })
          } else {
            const { parent, order } = slot(pages, source.id, drop)
            const values = { position: String(order), ...(parent && parent !== source.parent ? { parent } : {}) }
            write.mutate({ object: 'pages', record_id: source.id, values, ...(!parent && source.parent ? { remove: { parent: [] } } : {}) }, { onSuccess: () => moved(parent) })
          }
        }
        reset()
      },
    }),
  }
}

export type DataDrag = ReturnType<typeof useDataDrag>
