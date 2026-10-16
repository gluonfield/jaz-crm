import { type Editor, Extension, type Range } from '@tiptap/core'
import { type EditorState, PluginKey } from '@tiptap/pm/state'
import { ReactRenderer } from '@tiptap/react'
import Suggestion, { type SuggestionProps } from '@tiptap/suggestion'
import { type ReactNode, type Ref, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { cn } from '@/lib/utils'

type Look<T> = { label: string; empty: string; className: string; item: (item: T) => ReactNode }
type Keys = { key: (event: KeyboardEvent) => boolean }
type ListProps<T> = Look<T> & SuggestionProps<T, T> & { ref?: Ref<Keys> }

// suggestions opens a menu where char is typed outside code: find lists the
// matches for the text after it, and pick replaces that text with one.
export function suggestions<T>({ char, find, pick, allow = () => true, allowSpaces = false, debounce = 0, ...look }: Look<T> & {
  char: string
  find: (query: string) => T[] | Promise<T[]>
  pick: (editor: Editor, range: Range, item: T) => void
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
            let list: ReactRenderer<Keys, ListProps<T>> | undefined
            let unmount: (() => void) | undefined
            return {
              onStart: (props) => {
                list = new ReactRenderer(SuggestionList<T>, { editor: props.editor, props: { ...look, ...props }, className: 'z-50' })
                unmount = props.mount(list.element)
              },
              onUpdate: (props) => list?.updateProps(props),
              onKeyDown: ({ event }) => list?.ref?.key(event) ?? false,
              onExit: () => {
                unmount?.()
                list?.destroy()
              },
            }
          },
        }),
      ]
    },
  })
}

function SuggestionList<T>({ items, loading, command, label, empty, className, item, ref }: ListProps<T>) {
  // The highlight belongs to the matches it moved through; new matches start
  // at the first.
  const [moved, setMoved] = useState({ items, index: 0 })
  const index = moved.items === items ? moved.index : 0
  const list = useRef<HTMLDivElement>(null)
  useEffect(() => {
    list.current?.querySelector('[aria-selected=true]')?.scrollIntoView({ block: 'nearest' })
  }, [index])
  useImperativeHandle(ref, () => ({
    key: (event) => {
      if (!items.length) {
        return false
      }
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        setMoved({ items, index: (index + (event.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length })
        return true
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        command(items[index])
        return true
      }
      return false
    },
  }))
  return (
    <div ref={list} role="listbox" aria-label={label} className={cn('scrollbar-quiet max-h-96 overflow-y-auto rounded-[var(--radius-card)] border bg-raised p-1.5 text-[13px] shadow-[var(--shadow-raised)]', className)}>
      {items.length === 0 && <div className="px-2 py-2 text-[12px] text-ink-3">{loading ? 'Searching…' : empty}</div>}
      {items.map((option, at) => (
        <button
          key={at}
          type="button"
          role="option"
          aria-selected={at === index}
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => command(option)}
          className={cn('flex h-8 w-full items-center gap-2.5 rounded-[var(--radius-control)] px-2 text-left text-ink outline-none hover:bg-list-hover pointer-coarse:h-10', at === index && 'bg-list-active hover:bg-list-active')}
        >
          {item(option)}
        </button>
      ))}
    </div>
  )
}
