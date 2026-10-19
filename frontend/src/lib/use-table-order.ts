import { type DragEvent, useState } from 'react'
import { useAction } from './queries'
import type { CrmObject } from './types'

export function useTableOrder(object: CrmObject, defaults: string[]) {
  const edit = useAction<{ object: string; action: 'order'; columns: string[] }>('edit_object')
  const saved = edit.isPending ? edit.variables.columns : object.columns ?? []
  const all = [...new Set([...saved, ...defaults])]
  const order = all.filter((key) => defaults.includes(key))
  const [dragging, setDragging] = useState<string>()
  const [target, setTarget] = useState<{ key: string; after: boolean }>()
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
  const clear = () => {
    setDragging(undefined)
    setTarget(undefined)
  }
  const destination = (key: string, event: DragEvent<HTMLElement>) => {
    const rect = event.currentTarget.getBoundingClientRect()
    return { key, after: event.clientX >= rect.left + rect.width / 2 }
  }
  const header = (key: string) => ({
    draggable: !edit.isPending,
    onDragStart: (event: DragEvent<HTMLElement>) => {
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('text/plain', key)
      setDragging(key)
    },
    onDragOver: (event: DragEvent<HTMLElement>) => {
      if (!dragging) {
        return
      }
      event.preventDefault()
      event.dataTransfer.dropEffect = 'move'
      setTarget(destination(key, event))
    },
    onDrop: (event: DragEvent<HTMLElement>) => {
      if (!dragging) {
        return
      }
      event.preventDefault()
      const drop = destination(key, event)
      if (dragging !== key) {
        const others = order.filter((column) => column !== dragging)
        move(dragging, others.indexOf(key) + Number(drop.after))
      }
      clear()
    },
    onDragEnd: clear,
  })
  return { order, move, header, dragging, target, pending: edit.isPending }
}
