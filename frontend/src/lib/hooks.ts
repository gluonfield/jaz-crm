import { useEffect, useState } from 'react'

export function isTyping(e: KeyboardEvent) {
  const target = e.target as HTMLElement
  return (
    target.isContentEditable ||
    ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) ||
    !!target.closest('[role=dialog],[role=menu],[data-radix-popper-content-wrapper]')
  )
}

// useKeys runs single-key shortcuts while nothing takes typing.
export function useKeys(keys: Record<string, () => void>) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!isTyping(e) && !e.metaKey && !e.ctrlKey && !e.altKey && keys[e.key]) {
        e.preventDefault()
        keys[e.key]()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [keys])
}

// useListKeys moves a focused row with j/k or the arrows and hands other keys
// the focused index. Rows carry data-row={index} to be scrolled into view.
export function useListKeys(length: number, keys: Record<string, (index: number) => void> = {}) {
  const [cursor, setCursor] = useState(-1)
  const focus = Math.min(cursor, length - 1)
  const move = (delta: number) => {
    const next = Math.max(0, Math.min(length - 1, focus + delta))
    setCursor(next)
    document.querySelector(`[data-row="${next}"]`)?.scrollIntoView({ block: 'nearest' })
  }
  const bound: Record<string, () => void> = { j: () => move(1), ArrowDown: () => move(1), k: () => move(-1), ArrowUp: () => move(-1) }
  for (const [key, fn] of Object.entries(keys)) {
    bound[key] = () => focus >= 0 && fn(focus)
  }
  useKeys(bound)
  return [focus, setCursor] as const
}

export function useDebounced<T>(value: T, ms = 200) {
  const [settled, setSettled] = useState(value)
  useEffect(() => {
    const timer = window.setTimeout(() => setSettled(value), ms)
    return () => window.clearTimeout(timer)
  }, [value, ms])
  return settled
}
