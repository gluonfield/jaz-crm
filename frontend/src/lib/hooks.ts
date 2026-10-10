import { type RefObject, useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore } from 'react'

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
export function useListKeys(ids: string[], keys: Record<string, (index: number) => void> = {}) {
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
  const bound: Record<string, () => void> = { j: () => move(1), ArrowDown: () => move(1), k: () => move(-1), ArrowUp: () => move(-1) }
  for (const [key, fn] of Object.entries(keys)) {
    bound[key] = () => focus >= 0 && fn(focus)
  }
  useKeys(bound)
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

// useFlip slides the elements marked data-flip inside a container from where
// the last render laid them out to where they are now, so a list closes a gap
// or makes room instead of jumping.
export function useFlip(container: RefObject<HTMLElement | null>) {
  const last = useRef(new Map<string, number>())
  useLayoutEffect(() => {
    const still = matchMedia('(prefers-reduced-motion: reduce)').matches
    const next = new Map<string, number>()
    for (const el of container.current?.querySelectorAll<HTMLElement>('[data-flip]') ?? []) {
      const was = last.current.get(el.dataset.flip!)
      if (was !== undefined && was !== el.offsetTop && !still) {
        el.animate([{ transform: `translateY(${was - el.offsetTop}px)` }, { transform: 'none' }], { duration: 160, easing: 'cubic-bezier(0.2, 0, 0, 1)' })
      }
      next.set(el.dataset.flip!, el.offsetTop)
    }
    last.current = next
  })
}
