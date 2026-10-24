import { useAction } from './queries'
import type { CrmObject } from './types'

export function useTableOrder(object: CrmObject, defaults: string[]) {
  const edit = useAction<{ object: string; action: 'order'; columns: string[] }>('edit_object')
  const saved = edit.isPending ? edit.variables.columns : object.columns ?? []
  const all = [...new Set([...saved, ...defaults])]
  const order = all.filter((key) => defaults.includes(key))
  const move = (key: string, index: number) => {
    if (edit.isPending || !order.includes(key) || index < 0 || index >= order.length) {
      return
    }
    const columns = order.filter((column) => column !== key)
    columns.splice(index, 0, key)
    if (columns.some((column, i) => column !== order[i])) {
      // Keep archived columns in their saved slots while visible columns move.
      let next = 0
      edit.mutate({ object: object.slug, action: 'order', columns: all.map((column) => defaults.includes(column) ? columns[next++] : column) })
    }
  }
  return { order, move, pending: edit.isPending }
}
