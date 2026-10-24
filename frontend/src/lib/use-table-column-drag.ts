import { type PointerEvent as ReactPointerEvent, useEffect, useRef } from 'react'

export function useTableColumnDrag(move: (key: string, index: number) => void, disabled: boolean) {
  const cleanup = useRef<() => void>(undefined)
  useEffect(() => () => cleanup.current?.(), [])

  return (event: ReactPointerEvent<HTMLTableCellElement>) => {
    if (disabled || event.button !== 0 || cleanup.current || (event.target as HTMLElement).closest('button, input, [role="button"]')) {
      return
    }
    const header = event.currentTarget
    const table = header.closest('table')!
    const container = table.parentElement!
    const scroll = container.scrollLeft
    const columns = Array.from(table.tHead!.rows[0].cells)
      .filter((cell) => cell.dataset.column && cell.dataset.column !== 'name')
      .map((cell) => {
        const rect = cell.getBoundingClientRect()
        return { key: cell.dataset.column!, left: rect.left + scroll, width: rect.width, offset: NaN, cells: [] as HTMLTableCellElement[] }
      })
    for (const row of table.rows) {
      for (const cell of row.cells) {
        columns.find((column) => column.key === cell.dataset.column)?.cells.push(cell)
      }
    }
    const source = columns.find((column) => column.key === header.dataset.column)!
    const origin = event.clientX
    const cursor = document.body.style.cursor
    let x = origin
    let index = columns.indexOf(source)
    let frame = 0
    let overlay: HTMLDivElement | undefined
    let preview: HTMLDivElement | undefined

    const paint = (autoScroll = true) => {
      const bounds = container.getBoundingClientRect()
      const pinned = table.tHead!.rows[0].cells[0].getBoundingClientRect()
      if (autoScroll) {
        const edge = x < pinned.right + 40 ? x - pinned.right - 40 : x > bounds.right - 40 ? x - bounds.right + 40 : 0
        container.scrollLeft += Math.max(-12, Math.min(12, edge / 4))
      }
      const last = columns[columns.length - 1]
      const delta = Math.min(last.left + last.width - source.width - source.left,
        Math.max(pinned.right + container.scrollLeft - source.left, x - origin + container.scrollLeft - scroll))
      preview!.style.transform = `translate3d(${source.left - container.scrollLeft + delta - pinned.right}px, 0, 0)`
      const center = source.left + x - origin + container.scrollLeft - scroll + source.width / 2
      const others = columns.filter((column) => column !== source)
      const before = others.findIndex((column) => center < column.left + column.width / 2)
      index = before < 0 ? others.length : before
      others.splice(index, 0, source)
      let left = columns[0].left
      for (const column of others) {
        const offset = left - column.left
        if (column !== source && offset !== column.offset) {
          for (const cell of column.cells) {
            cell.style.transform = `translate3d(${offset}px, 0, 0)`
          }
          column.offset = offset
        }
        left += column.width
      }
      if (autoScroll) {
        frame = requestAnimationFrame(() => paint())
      }
    }
    const drag = (e: PointerEvent) => {
      if (e.pointerId !== event.pointerId) {
        return
      }
      x = e.clientX
      if (!preview && Math.abs(x - origin) >= 5) {
        const bounds = container.getBoundingClientRect()
        const pinned = table.tHead!.rows[0].cells[0].getBoundingClientRect()
        overlay = document.createElement('div')
        overlay.className = 'table-column-overlay'
        overlay.setAttribute('aria-hidden', 'true')
        overlay.inert = true
        Object.assign(overlay.style, {
          left: `${pinned.right}px`, top: `${pinned.top}px`, width: `${bounds.right - pinned.right}px`,
          height: `${Math.min(bounds.bottom, table.getBoundingClientRect().bottom) - pinned.top}px`,
        })
        preview = document.createElement('div')
        preview.className = 'table-column-preview'
        preview.style.width = `${source.width}px`
        const copy = table.cloneNode(true) as HTMLTableElement
        copy.style.width = `${source.width}px`
        copy.style.marginTop = `${table.getBoundingClientRect().top - pinned.top}px`
        const col = document.createElement('col')
        col.style.width = `${source.width}px`
        copy.querySelector('colgroup')!.replaceChildren(col)
        Array.from(copy.rows).forEach((row, i) => {
          row.style.height = `${table.rows[i].getBoundingClientRect().height}px`
          row.replaceChildren(Array.from(row.cells).find((cell) => cell.dataset.column === source.key)!)
        })
        preview.append(copy)
        overlay.append(preview)
        document.body.append(overlay)
        table.dataset.columnDragging = ''
        for (const cell of source.cells) {
          cell.dataset.columnMoving = ''
        }
        document.body.style.cursor = 'grabbing'
        header.setPointerCapture(e.pointerId)
        paint()
      }
    }
    const finish = () => {
      cancelAnimationFrame(frame)
      overlay?.remove()
      window.removeEventListener('pointermove', drag)
      window.removeEventListener('pointerup', drop)
      window.removeEventListener('pointercancel', cancel)
      window.removeEventListener('keydown', key, true)
      window.removeEventListener('blur', finish)
      window.removeEventListener('resize', finish)
      header.removeEventListener('lostpointercapture', finish)
      if (header.hasPointerCapture(event.pointerId)) {
        header.releasePointerCapture(event.pointerId)
      }
      delete table.dataset.columnDragging
      for (const column of columns) {
        for (const cell of column.cells) {
          delete cell.dataset.columnMoving
          cell.style.removeProperty('transform')
        }
      }
      document.body.style.cursor = cursor
      cleanup.current = undefined
    }
    const drop = (e: PointerEvent) => {
      if (e.pointerId !== event.pointerId) {
        return
      }
      if (preview) {
        x = e.clientX
        paint(false)
      }
      finish()
      if (preview) {
        move(source.key, index)
      }
    }
    const cancel = (e: PointerEvent) => {
      if (e.pointerId === event.pointerId) {
        finish()
      }
    }
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        finish()
      }
    }
    cleanup.current = finish
    window.addEventListener('pointermove', drag)
    window.addEventListener('pointerup', drop)
    window.addEventListener('pointercancel', cancel)
    window.addEventListener('keydown', key, true)
    window.addEventListener('blur', finish)
    window.addEventListener('resize', finish)
    header.addEventListener('lostpointercapture', finish)
  }
}
