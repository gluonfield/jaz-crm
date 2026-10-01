import { defaultParseSearch } from '@tanstack/react-router'

export function validateRecordSearch(search: Record<string, unknown>): { category?: string; sort?: 'name'; view?: 'table'; q?: string; where?: Record<string, string>; limit?: number } {
  return {
    view: search.view === 'table' ? 'table' : undefined,
    category: typeof search.category === 'string' ? search.category : undefined,
    sort: search.sort === 'name' ? 'name' : undefined,
    q: ['string', 'number', 'boolean'].includes(typeof search.q) ? String(search.q) : undefined,
    where: search.where && typeof search.where === 'object' && !Array.isArray(search.where) ? Object.fromEntries(Object.entries(search.where).filter(([, value]) => typeof value === 'string')) : undefined,
    limit: typeof search.limit === 'number' && Number.isInteger(search.limit) ? Math.max(1, Math.min(search.limit, 100)) : undefined,
  }
}

export function recordSearchInput(path: string) {
  const location = new URL(path, 'http://crm')
  const object = /^\/o\/([^/]+)$/.exec(location.pathname)?.[1] ?? ''
  const { q = '', where = {}, limit = 100, category } = validateRecordSearch(defaultParseSearch(location.search))
  return { object: decodeURIComponent(object), query: q.trim(), limit, where: { ...where, ...(category ? { categories: category } : {}) } }
}
