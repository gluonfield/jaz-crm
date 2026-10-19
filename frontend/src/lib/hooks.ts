import { type RefObject, useEffect, useState, useSyncExternalStore } from 'react'

import { useShortcutActions } from './shortcuts'
import type { ShortcutActions, ShortcutId } from './shortcut-definitions'

// useInView calls onView while active whenever the element comes near the
// visible area, such as the end of a list that loads more.
export function useInView(ref: RefObject<HTMLElement | null>, active: boolean, onView: () => void) {
  useEffect(() => {
    const element = ref.current
    if (!active || !element) {
      return
    }
    const observer = new IntersectionObserver((entries) => entries[0].isIntersecting && onView(), { rootMargin: '600px' })
    observer.observe(element)
    return () => observer.disconnect()
  }, [ref, active, onView])
}

// useListKeys moves a focused row with j/k or the arrows and hands other keys
// the focused index. Rows carry data-row={index} to be scrolled into view.
// Focus follows its item when the list reorders; when the item leaves, as
// when it is done, the item that took its place is focused.
export function useListKeys(ids: string[], keys: Partial<Record<ShortcutId, (index: number) => void>> = {}) {
  const [cursor, setCursor] = useState<{ id: string; index: number } | null>(null)
  const found = cursor ? ids.indexOf(cursor.id) : -1
  const focus = !cursor || ids.length === 0 ? -1 : found >= 0 ? found : Math.min(cursor.index, ids.length - 1)
  if (cursor && focus >= 0 && (found < 0 || found !== cursor.index)) {
    setCursor({ id: ids[focus], index: focus })
  }
  const select = (index: number) => setCursor(index >= 0 && index < ids.length ? { id: ids[index], index } : null)
  const move = (delta: number) => {
    const next = Math.max(0, Math.min(ids.length - 1, focus + delta))
    select(next)
    document.querySelector(`[data-row="${next}"]`)?.scrollIntoView({ block: 'nearest' })
  }
  const bound: ShortcutActions = { down: () => move(1), up: () => move(-1) }
  for (const [key, fn] of Object.entries(keys)) {
    if (fn) {
      bound[key as ShortcutId] = () => focus >= 0 && fn(focus)
    }
  }
  useShortcutActions(bound)
  return [focus, select] as const
}

// Phone width is below Tailwind's md breakpoint.
const phoneQuery = '(width < 48rem)'

function onPhoneChange(listener: () => void) {
  const query = matchMedia(phoneQuery)
  query.addEventListener('change', listener)
  return () => query.removeEventListener('change', listener)
}

// usePhone reports whether the app has phone width, where the sidebar is a
// drawer and tables are lists.
export function usePhone() {
  return useSyncExternalStore(onPhoneChange, () => matchMedia(phoneQuery).matches)
}

export function useDebounced<T>(value: T, ms = 200) {
  const [settled, setSettled] = useState(value)
  useEffect(() => {
    const timer = window.setTimeout(() => setSettled(value), ms)
    return () => window.clearTimeout(timer)
  }, [value, ms])
  return settled
}
