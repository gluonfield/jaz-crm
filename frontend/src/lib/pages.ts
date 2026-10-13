import { type DragEvent, useMemo, useState } from 'react'
import { recordName, valuesOf } from './crm'
import { useAction, useObjects, useTool } from './queries'
import type { CrmObject, CrmRecord, Ref } from './types'

export type Page = { id: string; name: string; parent?: string; icon?: string }

export const pageIcon = (record: CrmRecord) => record.object === 'pages' && typeof record.values.icon === 'string' ? record.values.icon : undefined

// Pages is every page, its sub-pages by parent ('' for the top level), oldest
// first.
export type Pages = { byId: Map<string, Page>; children: Map<string, Page[]>; tables: Map<string, CrmObject[]> }

export function usePages(): Pages | undefined {
  const records = useTool<{ records: CrmRecord[] }>('search_records', { object: 'pages', limit: 500 }).data?.records
  const objects = useObjects()
  return useMemo(() => records && objects && pagesOf(records, objects), [records, objects])
}

function pagesOf(records: CrmRecord[], objects: CrmObject[]): Pages {
  const byId = new Map<string, Page>()
  for (const r of [...records].sort((a, b) => a.created_at.localeCompare(b.created_at))) {
    byId.set(r.id, { id: r.id, name: recordName(r), parent: (valuesOf(r, 'parent')[0] as Ref | undefined)?.id, icon: pageIcon(r) })
  }
  const children = new Map<string, Page[]>()
  // A page whose parent is unknown, or which no top-level page reaches, sits
  // at the top level.
  const top = (page: Page) => !page.parent || !byId.has(page.parent) || path({ byId }, page.id).length === 0
  for (const page of byId.values()) {
    const key = top(page) ? '' : page.parent!
    children.set(key, [...(children.get(key) ?? []), page])
  }
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

export function useDataDrag(pages: Pages | undefined, moved: (parent: string) => void) {
  const write = useAction<object>('upsert_record')
  const edit = useAction<object>('edit_object')
  const [source, setSource] = useState<Page | CrmObject>()
  const [target, setTarget] = useState<string>()
  const type = 'application/x-jaz-crm-data'
  const pending = write.isPending || edit.isPending
  const key = (item: Page | CrmObject) => 'slug' in item ? `database:${item.slug}` : `page:${item.id}`
  const reset = () => {
    setSource(undefined)
    setTarget(undefined)
  }
  const accepts = (parent: string) => {
    return !!pages && !!source && !pending && (parent === '' || pages.byId.has(parent)) && parent !== (source.parent ?? '') && ('slug' in source || !within(pages, parent, source.id))
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
    onto: (parent: string) => ({
      onDragOver: (event: DragEvent<HTMLElement>) => {
        if (!event.dataTransfer.types.includes(type)) {
          return
        }
        event.stopPropagation()
        const allowed = accepts(parent)
        event.dataTransfer.dropEffect = allowed ? 'move' : 'none'
        setTarget(allowed ? parent : undefined)
        if (allowed) {
          event.preventDefault()
        }
      },
      onDragLeave: (event: DragEvent<HTMLElement>) => {
        if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) {
          setTarget((current) => current === parent ? undefined : current)
        }
      },
      onDrop: (event: DragEvent<HTMLElement>) => {
        if (!event.dataTransfer.types.includes(type)) {
          return
        }
        event.preventDefault()
        event.stopPropagation()
        if (source && event.dataTransfer.getData(type) === key(source) && accepts(parent)) {
          const callbacks = { onSuccess: () => moved(parent) }
          if ('slug' in source) {
            edit.mutate({ object: source.slug, action: 'move', parent }, callbacks)
          } else {
            write.mutate({ object: 'pages', record_id: source.id, ...(parent ? { values: { parent } } : { remove: { parent: [] } }) }, callbacks)
          }
        }
        reset()
      },
    }),
  }
}

export type DataDrag = ReturnType<typeof useDataDrag>
