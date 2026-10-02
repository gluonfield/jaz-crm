import { type PointerEvent, useRef, useState } from 'react'
import { readItem, writeItem } from './storage'

const key = 'crm-column-widths'

type Widths = Record<string, Record<string, number>>

// useColumnWidths keeps the widths people drag table columns to, per object,
// across visits.
export function useColumnWidths(object: string) {
  const [widths, setWidths] = useState<Widths>(() => JSON.parse(readItem(key) ?? '{}'))
  const latest = useRef(widths)
  const width = (column: string, fallback: number) => widths[object]?.[column] ?? fallback
  const resize = (column: string, fallback: number) => (e: PointerEvent) => {
    e.preventDefault()
    e.stopPropagation()
    const from = e.clientX
    const start = width(column, fallback)
    const move = (m: globalThis.PointerEvent) => {
      latest.current = { ...latest.current, [object]: { ...latest.current[object], [column]: Math.max(80, Math.round(start + m.clientX - from)) } }
      setWidths(latest.current)
    }
    const up = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      writeItem(key, JSON.stringify(latest.current))
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }
  return { width, resize }
}
