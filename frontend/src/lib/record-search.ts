import { z } from 'zod'
import { defaultParseSearch } from '@tanstack/react-router'
import { filterOperators, type RecordFilter } from './types'

export type RecordSearchScope = { group_by_conversation?: boolean; conversation_id?: string }

export function validateRecordSearch(search: Record<string, unknown>): RecordSearchScope & { sort?: 'name'; view?: 'table' | 'board'; q?: string; filters?: RecordFilter[]; saved?: string; limit?: number; where?: never; category?: never } {
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
    group_by_conversation: typeof search.group_by_conversation === 'boolean' ? search.group_by_conversation : undefined,
    conversation_id: typeof search.conversation_id === 'string' ? search.conversation_id : undefined,
  }
}

export function recordSearchInput(path: string) {
  const location = new URL(path, 'http://crm')
  const object = /^\/o\/([^/]+)$/.exec(location.pathname)?.[1] ?? ''
  const { q = '', filters = [], limit = 100, group_by_conversation, conversation_id } = validateRecordSearch(defaultParseSearch(location.search))
  return { object: decodeURIComponent(object), query: q.trim(), limit, filters, group_by_conversation, conversation_id }
}
