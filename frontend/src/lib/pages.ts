import { type DragEvent, useMemo, useState } from 'react'
import { recordName, valuesOf } from './crm'
import { useAction, useObjects, useTool } from './queries'
import type { CrmObject, CrmRecord, Ref } from './types'

// A page's position sorts it among the pages and tables beside it: its stored
// position, else when it was made. parent is where it sits in the tree.
export type Page = { id: string; name: string; parent?: string; icon?: string; position: number }

// Item is a page or a table in the Data tree.
export type Item = Page | CrmObject

export const isTable = (item: Item): item is CrmObject => 'slug' in item

// keyOf names an item's row, '' for the top level's.
const keyOf = (item?: Item) => !item ? '' : isTable(item) ? `database:${item.slug}` : `page:${item.id}`

export const pageIcon = (record: CrmRecord) => record.object === 'pages' && typeof record.values.icon === 'string' ? record.values.icon : undefined

// Pages is every page, and the pages and tables inside each page ('' for the
// top level) in order.
export type Pages = { byId: Map<string, Page>; children: Map<string, Item[]> }

export function usePages(): Pages | undefined {
  const records = useTool<{ records: CrmRecord[] }>('search_records', { object: 'pages', limit: 500 }).data?.records
  const objects = useObjects()
  return useMemo(() => records && objects && pagesOf(records, objects), [records, objects])
}

export function pagesOf(records: CrmRecord[], objects: CrmObject[]): Pages {
  const byId = new Map<string, Page>()
  for (const r of [...records].sort((a, b) => a.created_at.localeCompare(b.created_at))) {
    const position = valuesOf(r, 'position')[0]
    byId.set(r.id, { id: r.id, name: recordName(r), parent: (valuesOf(r, 'parent')[0] as Ref | undefined)?.id, icon: pageIcon(r), position: typeof position === 'string' ? Number(position) : Date.parse(r.created_at) })
  }
  // An item whose parent is unknown, or a page which no top-level page
  // reaches, sits at the top level.
  const top = (item: Item) => !item.parent || !byId.has(item.parent) || (!isTable(item) && path({ byId }, item.id).length === 0)
  const tables = objects.filter((object) => !object.standard).map((table) => ({ ...table }))
  const items = [...tables, ...byId.values()].sort((a, b) => a.position - b.position)
  const parents = items.map((item) => (top(item) ? '' : item.parent!))
  const children = new Map<string, Item[]>()
  items.forEach((item, i) => {
    item.parent = parents[i] || undefined
    children.set(parents[i], [...(children.get(parents[i]) ?? []), item])
  })
  return { byId, children }
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

// Drop is where a moved item lands: inside a page (none for the top level),
// or before or after a page or table among its siblings.
export type Drop = { place: 'inside'; page?: Page } | { place: 'before' | 'after'; item: Item }
export type Place = Drop['place']
type Edge = Exclude<Place, 'inside'>

// parentOf is the page a drop puts an item in, '' for the top level.
const parentOf = (drop: Drop) => (drop.place === 'inside' ? (drop.page?.id ?? '') : (drop.item.parent ?? ''))

// rowOf names the row a drop is at.
const rowOf = (drop: Drop) => keyOf(drop.place === 'inside' ? drop.page : drop.item)

// slot is where an item dropped there sits: its parent, and a position between
// its new neighbours, after the last item when it goes inside.
export function slot(pages: Pages, moving: Item, drop: Drop) {
  const parent = parentOf(drop)
  const siblings = (pages.children.get(parent) ?? []).filter((item) => keyOf(item) !== keyOf(moving))
  const at = drop.place === 'inside' ? siblings.length : siblings.findIndex((item) => keyOf(item) === keyOf(drop.item)) + (drop.place === 'after' ? 1 : 0)
  const [before, after] = [siblings[at - 1], siblings[at]]
  const position = before && after ? (before.position + after.position) / 2 : before ? before.position + 1000 : after ? after.position - 1000 : 0
  return { parent, position }
}

// allowed reports whether a drop moves an item somewhere new, never a page
// into its own branch.
export function allowed(pages: Pages, item: Item, drop: Drop) {
  const parent = parentOf(drop)
  if (drop.place === 'inside' ? parent === (item.parent ?? '') : keyOf(drop.item) === keyOf(item)) {
    return false
  }
  return isTable(item) || !within(pages, parent, item.id)
}

// useMove moves a page or table to where a drop puts it. A move leaves out an
// unchanged parent, so an item whose parent is in Trash keeps it.
export function useMove() {
  const write = useAction<object>('upsert_record')
  const edit = useAction<object>('edit_object')
  return {
    pending: write.isPending || edit.isPending,
    move: (pages: Pages, item: Item, drop: Drop, moved?: (parent: string) => void) => {
      if (!allowed(pages, item, drop)) {
        return
      }
      const { parent, position } = slot(pages, item, drop)
      const done = { onSuccess: () => moved?.(parent) }
      if (isTable(item)) {
        edit.mutate({ object: item.slug, action: 'move', position, ...(parent !== (item.parent ?? '') ? { parent } : {}) }, done)
      } else {
        const values = { position: String(position), ...(parent && parent !== item.parent ? { parent } : {}) }
        write.mutate({ object: 'pages', record_id: item.id, values, ...(!parent && item.parent ? { remove: { parent: [] } } : {}) }, done)
      }
    },
  }
}

export function useDataDrag(pages: Pages | undefined, moved: (parent: string) => void) {
  const { pending, move } = useMove()
  const [source, setSource] = useState<Item>()
  const [target, setTarget] = useState<Drop>()
  const type = 'application/x-jaz-crm-data'
  const reset = () => {
    setSource(undefined)
    setTarget(undefined)
  }
  // A row's edges place an item before or after it: a table's halves, or a
  // page's top and bottom thirds where it offers them, its middle taking the
  // item inside.
  const dropAt = (event: DragEvent<HTMLElement>, item: Item | undefined, edges: Edge[]): Drop => {
    const { top, height } = event.currentTarget.getBoundingClientRect()
    const y = (event.clientY - top) / height
    if (item && isTable(item)) {
      return { place: y < 0.5 ? 'before' : 'after', item }
    }
    const edge = y < 0.3 ? 'before' : y >= 0.7 ? 'after' : 'inside'
    return item && edge !== 'inside' && edges.includes(edge) ? { place: edge, item } : { place: 'inside', page: item }
  }
  const accepts = (drop: Drop) => !!pages && !!source && !pending && allowed(pages, source, drop)
  return {
    dragging: !!source,
    // at is where a dragged item would land at an item's row, or the top
    // level's.
    at: (item?: Item) => (target && rowOf(target) === keyOf(item) ? target.place : undefined),
    from: (item: Item) => ({
      draggable: !pending,
      'data-dragging': !!source && keyOf(source) === keyOf(item),
      onDragStart: (event: DragEvent<HTMLElement>) => {
        if (pending) {
          event.preventDefault()
          return
        }
        event.dataTransfer.clearData()
        event.dataTransfer.setData(type, keyOf(item))
        event.dataTransfer.effectAllowed = 'move'
        setSource(item)
      },
      onDragEnd: reset,
    }),
    // onto makes an item's row, or the top level's, a drop target; a page row
    // offers the given edges.
    onto: (item?: Item, edges: Edge[] = []) => ({
      onDragOver: (event: DragEvent<HTMLElement>) => {
        if (!event.dataTransfer.types.includes(type)) {
          return
        }
        event.stopPropagation()
        const drop = dropAt(event, item, edges)
        const allowed = accepts(drop)
        event.dataTransfer.dropEffect = allowed ? 'move' : 'none'
        setTarget((current) => !allowed ? undefined : current && rowOf(current) === rowOf(drop) && current.place === drop.place ? current : drop)
        if (allowed) {
          event.preventDefault()
        }
      },
      onDragLeave: (event: DragEvent<HTMLElement>) => {
        if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) {
          setTarget((current) => current && rowOf(current) === keyOf(item) ? undefined : current)
        }
      },
      onDrop: (event: DragEvent<HTMLElement>) => {
        if (!event.dataTransfer.types.includes(type)) {
          return
        }
        event.preventDefault()
        event.stopPropagation()
        const drop = dropAt(event, item, edges)
        if (pages && source && event.dataTransfer.getData(type) === keyOf(source) && accepts(drop)) {
          move(pages, source, drop, moved)
        }
        reset()
      },
    }),
  }
}

export type DataDrag = ReturnType<typeof useDataDrag>
