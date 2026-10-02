import { useMemo } from 'react'
import { recordName, valuesOf } from './crm'
import { useTool } from './queries'
import type { CrmRecord, Ref } from './types'

export type Page = { id: string; name: string; parent?: string }

// Pages is every page, its sub-pages by parent ('' for the top level), oldest
// first.
export type Pages = { byId: Map<string, Page>; children: Map<string, Page[]> }

export function usePages(): Pages | undefined {
  const records = useTool<{ records: CrmRecord[] }>('search_records', { object: 'pages', limit: 500 }).data?.records
  return useMemo(() => records && pagesOf(records), [records])
}

function pagesOf(records: CrmRecord[]): Pages {
  const byId = new Map<string, Page>()
  for (const r of [...records].sort((a, b) => a.created_at.localeCompare(b.created_at))) {
    byId.set(r.id, { id: r.id, name: recordName(r), parent: (valuesOf(r, 'parent')[0] as Ref | undefined)?.id })
  }
  const children = new Map<string, Page[]>()
  // A page whose parent is unknown, or which no top-level page reaches, sits
  // at the top level.
  const top = (page: Page) => !page.parent || !byId.has(page.parent) || path({ byId, children }, page.id).length === 0
  for (const page of byId.values()) {
    const key = top(page) ? '' : page.parent!
    children.set(key, [...(children.get(key) ?? []), page])
  }
  return { byId, children }
}

// path lists a page's ancestors and the page from the top, or nothing when
// its parents loop.
export function path(pages: Pages, id: string): Page[] {
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
