import { z } from 'zod'
import { defaultParseSearch } from '@tanstack/react-router'
import { filterOperators, type RecordFilter } from './types'

export function validateRecordSearch(search: Record<string, unknown>): { sort?: 'name'; view?: 'table' | 'board'; q?: string; filters?: RecordFilter[]; saved?: string; limit?: number; where?: never; category?: never } {
  const filters = z.array(z.object({ attribute: z.string(), operator: z.enum(filterOperators), value: z.string().optional() })).parse(search.filters ?? [])
  if (search.where && typeof search.where === 'object' && !Array.isArray(search.where)) {
    for (const [attribute, value] of Object.entries(search.where)) {
      if (typeof value === 'string') {
        filters.push({ attribute, operator: 'is', value })
      }
    }
  }
  if (typeof search.category === 'string' && search.category) {
    filters.push({ attribute: 'categories', operator: 'is', value: search.category })
  }
  return {
    where: undefined,
    category: undefined,
    view: search.view === 'table' || search.view === 'board' ? search.view : undefined,
    sort: search.sort === 'name' ? 'name' : undefined,
    q: ['string', 'number', 'boolean'].includes(typeof search.q) ? String(search.q) : undefined,
    filters: filters.length || Array.isArray(search.filters) ? filters : undefined,
    saved: typeof search.saved === 'string' ? search.saved : undefined,
    limit: typeof search.limit === 'number' && Number.isInteger(search.limit) ? Math.max(1, Math.min(search.limit, 100)) : undefined,
  }
}

export function recordSearchInput(path: string) {
  const location = new URL(path, 'http://crm')
  const object = /^\/o\/([^/]+)$/.exec(location.pathname)?.[1] ?? ''
  const { q = '', filters = [], limit = 100 } = validateRecordSearch(defaultParseSearch(location.search))
  return { object: decodeURIComponent(object), query: q.trim(), limit, filters }
}
