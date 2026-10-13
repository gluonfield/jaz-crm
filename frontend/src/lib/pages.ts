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

export function usePageDrag(pages: Pages | undefined, moved: (parent: string) => void) {
  const write = useAction<object>('upsert_record')
  const [dragging, setDragging] = useState<string>()
  const [target, setTarget] = useState<string>()
  const type = 'application/x-jaz-crm-page'
  const reset = () => {
    setDragging(undefined)
    setTarget(undefined)
  }
  const accepts = (parent: string) => {
    const page = pages?.byId.get(dragging ?? '')
    return !!pages && !!page && (parent === '' || pages.byId.has(parent)) && parent !== (page.parent ?? '') && !within(pages, parent, page.id)
  }
  return {
    dragging,
    target,
    from: (id: string) => ({
      draggable: !write.isPending,
      onDragStart: (event: DragEvent<HTMLElement>) => {
        if (write.isPending) {
          event.preventDefault()
          return
        }
        event.dataTransfer.clearData()
        event.dataTransfer.setData(type, id)
        event.dataTransfer.effectAllowed = 'move'
        setDragging(id)
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
        if (allowed) {
          event.preventDefault()
          setTarget(parent)
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
        if (event.dataTransfer.getData(type) === dragging && accepts(parent)) {
          write.mutate(
            { object: 'pages', record_id: dragging, ...(parent ? { values: { parent } } : { remove: { parent: [] } }) },
            { onSuccess: () => moved(parent) },
          )
        }
        reset()
      },
    }),
  }
}

export type PageDrag = ReturnType<typeof usePageDrag>
