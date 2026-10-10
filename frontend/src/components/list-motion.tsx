import { Component, createRef, type ReactNode, type RefObject } from 'react'

type Row = { element: HTMLElement; rect: DOMRect; clip: DOMRect; headerBottom: number; background: string; opacity: string; columns: number[] }
type Props = { children: ReactNode; className?: string; scope?: string; container?: RefObject<HTMLDivElement | null> }

// React's snapshot lifecycle measures the painted rows before removal, even
// after scrolling or an interrupted animation. Exits retain only inert pixels.
export class ListMotion extends Component<Props, object, Map<string, Row> | null> {
  private root = createRef<HTMLDivElement>()
  private animations = new Set<Animation>()
  private exits = new Set<HTMLElement>()

  private rows() {
    const root = (this.props.container ?? this.root).current!
    return [...root.querySelectorAll<HTMLElement>('[data-flip]')].filter((element) => element.closest('[data-list-motion]') === root)
  }

  getSnapshotBeforeUpdate(previous: Props) {
    if (previous.scope !== this.props.scope || matchMedia('(prefers-reduced-motion: reduce)').matches) {
      this.clear()
      return null
    }
    return new Map(this.rows().map((element) => [element.dataset.flip!, {
      element,
      rect: element.getBoundingClientRect(),
      clip: (element.closest('.scrollbar-quiet') ?? (this.props.container ?? this.root).current!).getBoundingClientRect(),
      headerBottom: element.tagName === 'TR' ? element.closest('table')?.querySelector('thead')?.getBoundingClientRect().bottom ?? 0 : 0,
      background: getComputedStyle(element).backgroundColor,
      opacity: getComputedStyle(element).opacity,
      columns: element.tagName === 'TR' ? [...element.children].map((cell) => cell.getBoundingClientRect().width) : [],
    }]))
  }

  componentDidUpdate(_previous: Props, _state: object, before: Map<string, Row> | null) {
    if (!before?.size) {
      return
    }
    const root = (this.props.container ?? this.root).current!
    const rows = this.rows()
    if ([...before.keys()].join('\n') === rows.map((row) => row.dataset.flip).join('\n') && rows.every((row) => before.get(row.dataset.flip!)?.element === row)) {
      return
    }
    const bounds = root.getBoundingClientRect()
    for (const element of rows) {
      const was = before.get(element.dataset.flip!)
      element.getAnimations().filter((animation) => this.animations.has(animation)).forEach((animation) => animation.cancel())
      const rect = element.getBoundingClientRect()
      if (was && Math.abs(was.rect.width - rect.width) < 1) {
        const x = was.rect.left - rect.left
        const y = was.rect.top - rect.top
        if (x || y) {
          this.play(element, [{ transform: `translate(${x}px, ${y}px)` }, { transform: 'none' }])
        }
      } else if (!was) {
        this.play(element, [{ opacity: 0 }, { opacity: 1 }])
      }
      before.delete(element.dataset.flip!)
    }
    for (const { element, rect, clip, headerBottom, background, opacity, columns } of before.values()) {
      const top = Math.max(bounds.top, clip.top, headerBottom)
      const bottom = Math.min(bounds.bottom, clip.bottom)
      const left = Math.max(bounds.left, clip.left)
      const right = Math.min(bounds.right, clip.right)
      if (rect.bottom <= top || rect.top >= bottom || rect.right <= left || rect.left >= right) {
        continue
      }
      const exit = element.cloneNode(true) as HTMLElement
      exit.removeAttribute('data-flip')
      exit.removeAttribute('data-row')
      exit.inert = true
      exit.setAttribute('aria-hidden', 'true')
      Object.assign(exit.style, { position: 'absolute', margin: '0', left: `${rect.left - bounds.left + root.scrollLeft}px`, top: `${rect.top - bounds.top + root.scrollTop}px`, width: `${rect.width}px`, height: `${rect.height}px`, background, pointerEvents: 'none', zIndex: '20', clipPath: `inset(${Math.max(0, top - rect.top)}px ${Math.max(0, rect.right - right)}px ${Math.max(0, rect.bottom - bottom)}px ${Math.max(0, left - rect.left)}px)` })
      if (columns.length) {
        Object.assign(exit.style, { display: 'table', tableLayout: 'fixed' })
        columns.forEach((width, index) => (exit.children[index] as HTMLElement).style.width = `${width}px`)
      }
      root.append(exit)
      this.exits.add(exit)
      this.play(exit, [{ opacity, transform: 'none' }, { opacity: 0, transform: 'translateY(-4px)' }], () => {
        exit.remove()
        this.exits.delete(exit)
      })
    }
  }

  componentWillUnmount() {
    this.clear()
  }

  private play(element: HTMLElement, frames: Keyframe[], finished?: () => void) {
    const animation = element.animate(frames, { duration: 160, easing: 'cubic-bezier(0.2, 0, 0, 1)' })
    this.animations.add(animation)
    animation.finished.catch(() => {}).finally(() => {
      this.animations.delete(animation)
      finished?.()
    })
  }

  private clear() {
    this.animations.forEach((animation) => animation.cancel())
    this.exits.forEach((exit) => exit.remove())
    this.animations.clear()
    this.exits.clear()
  }

  render() {
    return <div ref={this.props.container ?? this.root} data-list-motion className={this.props.className} style={{ position: 'relative' }}>{this.props.children}</div>
  }
}
