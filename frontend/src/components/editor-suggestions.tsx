import { type Editor, Extension, type Range } from '@tiptap/core'
import { type EditorState, PluginKey } from '@tiptap/pm/state'
import Suggestion, { type SuggestionProps } from '@tiptap/suggestion'
import { type ReactNode, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { cn } from '@/lib/utils'

type Menu<T> = { items: T[]; index: number; loading: boolean; pick: (item: T) => void; element: HTMLElement }
type Bridge<T> = { show: (props: SuggestionProps<T, T>, element?: HTMLElement) => void; key: (event: KeyboardEvent) => boolean; hide: () => void }

// suggestions opens a menu where char is typed outside code: find lists the
// matches for the text after it, and pick replaces that text with one.
export function suggestions<T>({ char, find, pick, bridge, allow = () => true, allowSpaces = false, debounce = 0 }: {
  char: string
  find: (query: string) => T[] | Promise<T[]>
  pick: (editor: Editor, range: Range, item: T) => void
  bridge: Bridge<T>
  allow?: (state: EditorState, range: Range) => boolean
  allowSpaces?: boolean
  debounce?: number
}) {
  const name = `suggestions${char}`
  return Extension.create({
    name,
    addProseMirrorPlugins() {
      return [
        Suggestion<T, T>({
          editor: this.editor,
          pluginKey: new PluginKey(name),
          char,
          allowSpaces,
          debounce,
          allow: ({ state, range }) => !state.doc.resolve(range.from).parent.type.spec.code && allow(state, range),
          items: ({ query }) => find(query),
          command: ({ editor, range, props }) => pick(editor, range, props),
          render: () => {
            let unmount: (() => void) | undefined
            return {
              onStart: (props) => {
                const element = document.createElement('div')
                element.className = 'z-50'
                unmount = props.mount(element)
                bridge.show(props, element)
              },
              onUpdate: (props) => bridge.show(props),
              onKeyDown: ({ event }) => bridge.key(event),
              onExit: () => {
                unmount?.()
                bridge.hide()
              },
            }
          },
        }),
      ]
    },
  })
}

export function useSuggestionMenu<T>() {
  const [menu, setMenu] = useState<Menu<T> | null>(null)
  const current = useRef<Menu<T> | null>(null)
  const [bridge] = useState<Bridge<T>>(() => {
    const set = (next: Menu<T> | null) => {
      current.current = next
      setMenu(next)
    }
    return {
      show: (props, element) => set({ items: props.items, index: 0, loading: props.loading, pick: props.command, element: element ?? current.current!.element }),
      key: (event) => {
        const open = current.current
        if (!open?.items.length) {
          return false
        }
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
          set({ ...open, index: (open.index + (event.key === 'ArrowDown' ? 1 : open.items.length - 1)) % open.items.length })
          return true
        }
        if (event.key === 'Enter' || event.key === 'Tab') {
          open.pick(open.items[open.index])
          return true
        }
        return false
      },
      hide: () => set(null),
    }
  })
  return [menu, bridge] as const
}

export function SuggestionList<T>({ menu, label, empty, className, children }: { menu: Menu<T>; label: string; empty: string; className: string; children: (item: T) => ReactNode }) {
  const list = useRef<HTMLDivElement>(null)
  useEffect(() => {
    list.current?.querySelector('[aria-selected=true]')?.scrollIntoView({ block: 'nearest' })
  }, [menu.index])
  return createPortal(
    <div ref={list} role="listbox" aria-label={label} className={cn('scrollbar-quiet max-h-96 overflow-y-auto rounded-[var(--radius-card)] border bg-raised p-1.5 text-[13px] shadow-[var(--shadow-raised)]', className)}>
      {menu.items.length === 0 && <div className="px-2 py-2 text-[12px] text-ink-3">{menu.loading ? 'Searching…' : empty}</div>}
      {menu.items.map((item, index) => (
        <button
          key={index}
          type="button"
          role="option"
          aria-selected={index === menu.index}
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => menu.pick(item)}
          className={cn('flex h-8 w-full items-center gap-2.5 rounded-[var(--radius-control)] px-2 text-left text-ink outline-none hover:bg-list-hover pointer-coarse:h-10', index === menu.index && 'bg-list-active hover:bg-list-active')}
        >
          {children(item)}
        </button>
      ))}
    </div>,
    menu.element,
  )
}
