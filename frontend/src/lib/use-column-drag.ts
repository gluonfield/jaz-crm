import { type PointerEvent as ReactPointerEvent, useEffect, useRef, useState } from 'react'

export function useColumnDrag(options: string[], save: (stage: string, before?: string) => Promise<unknown>) {
  const boardRef = useRef<HTMLDivElement>(null)
  const cleanup = useRef<() => void>(undefined)
  const [view, setView] = useState<{ stage: string; offsets: Record<string, number> }>()
  const [settled, setSettled] = useState<{ from: string[]; to: string[] }>()
  const order = settled && options.length === settled.from.length && options.every((s, i) => s === settled.from[i]) ? settled.to : options
  useEffect(() => () => cleanup.current?.(), [])

  const start = (stage: string, event: ReactPointerEvent<HTMLButtonElement>) => {
    if (event.button !== 0 || cleanup.current) {
      return
    }
    const container = boardRef.current!
    const columns = Array.from(container.querySelectorAll<HTMLElement>('[data-stage]')).map((node) => ({ node, stage: node.dataset.stage!, rect: node.getBoundingClientRect() }))
    const source = columns.find((column) => column.stage === stage)!
    const gap = parseFloat(getComputedStyle(container).columnGap)
    const scroll = container.scrollLeft
    const cursor = container.style.cursor
    let x = event.clientX
    let y = event.clientY
    const origin = { x, y }
    let next: string[] = []
    let ghost: HTMLElement | undefined
    let frame = 0

    const paint = () => {
      const bounds = container.getBoundingClientRect()
      const edge = x < bounds.left + 40 ? x - bounds.left - 40 : x > bounds.right - 40 ? x - bounds.right + 40 : 0
      container.scrollLeft += Math.max(-12, Math.min(12, edge / 4))
      ghost!.style.transform = `translate3d(${x - origin.x}px, ${y - origin.y}px, 0)`
      const center = source.rect.left + x - origin.x + source.rect.width / 2 + container.scrollLeft - scroll
      const others = columns.filter((column) => column.stage !== stage)
      const before = others.findIndex((column) => center < column.rect.left + column.rect.width / 2)
      const reordered = others.map((column) => column.stage)
      reordered.splice(before < 0 ? others.length : before, 0, stage)
      if (reordered.some((s, i) => s !== next[i])) {
        next = reordered
        const offsets: Record<string, number> = {}
        let left = columns[0].rect.left
        for (const name of next) {
          const column = columns.find((c) => c.stage === name)!
          offsets[name] = left - column.rect.left
          left += column.rect.width + gap
        }
        setView({ stage, offsets })
      }
      frame = requestAnimationFrame(paint)
    }
    const move = (e: PointerEvent) => {
      if (e.pointerId !== event.pointerId) {
        return
      }
      x = e.clientX
      y = e.clientY
      if (!ghost && Math.hypot(x - origin.x, y - origin.y) >= 5) {
        ghost = source.node.cloneNode(true) as HTMLElement
        ghost.removeAttribute('data-stage')
        ghost.setAttribute('aria-hidden', 'true')
        ghost.inert = true
        Object.assign(ghost.style, {
          position: 'fixed', left: `${source.rect.left}px`, top: `${source.rect.top}px`,
          width: `${source.rect.width}px`, height: `${source.rect.height}px`, zIndex: '100',
          background: 'var(--color-raised)', boxShadow: '0 0 0 1px var(--color-border), 0 16px 48px rgb(0 0 0 / 0.22)',
          pointerEvents: 'none', transition: 'none',
        })
        document.body.append(ghost)
        const list = source.node.querySelector('ol')
        if (list) {
          ghost.querySelector('ol')!.scrollTop = list.scrollTop
        }
        container.setPointerCapture(e.pointerId)
        container.style.cursor = 'grabbing'
        paint()
      }
    }
    const finish = () => {
      cancelAnimationFrame(frame)
      ghost?.remove()
      container.style.cursor = cursor
      if (container.hasPointerCapture(event.pointerId)) {
        container.releasePointerCapture(event.pointerId)
      }
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', end)
      window.removeEventListener('pointercancel', cancel)
      window.removeEventListener('keydown', key)
      window.removeEventListener('blur', finish)
      cleanup.current = undefined
      setView(undefined)
    }
    const end = (e: PointerEvent) => {
      if (e.pointerId !== event.pointerId) {
        return
      }
      if (ghost) {
        cancelAnimationFrame(frame)
        x = e.clientX
        y = e.clientY
        paint()
      }
      finish()
      if (ghost && next.some((s, i) => s !== order[i])) {
        setSettled({ from: options, to: next })
        void save(stage, next[next.indexOf(stage) + 1]).finally(() => setSettled(undefined)).catch(() => {})
      }
    }
    const cancel = (e: PointerEvent) => {
      if (e.pointerId === event.pointerId) {
        finish()
      }
    }
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        finish()
      }
    }
    cleanup.current = finish
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', end)
    window.addEventListener('pointercancel', cancel)
    window.addEventListener('keydown', key)
    window.addEventListener('blur', finish)
  }
  return { boardRef, order, view, start }
}
