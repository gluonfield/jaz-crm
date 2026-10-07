import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { type Editor, Extension } from '@tiptap/core'
import { TaskItem, TaskList } from '@tiptap/extension-list'
import { TableKit } from '@tiptap/extension-table'
import { Placeholder } from '@tiptap/extensions'
import { Markdown } from '@tiptap/markdown'
import { EditorContent, useEditor } from '@tiptap/react'
import Suggestion, { type SuggestionProps } from '@tiptap/suggestion'
import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { toast } from 'sonner'
import { call, embedded } from '@/lib/api'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { app } from '@/lib/mcp-app'
import { type Pages, path, usePages } from '@/lib/pages'
import { toolQuery, useObjects } from '@/lib/queries'
import type { CrmObject, CrmRecord } from '@/lib/types'
import { cn } from '@/lib/utils'
import { singular } from './create-record'
import { DocumentKit } from './document-links'
import { RecordIcon } from './icons'

type Mention = { id: string; name: string; object: string; detail: string; photo?: string; icon?: string }
type Menu = { items: Mention[]; index: number; loading: boolean; pick: (item: Mention) => void; element: HTMLElement }
type Bridge = { show: (props: SuggestionProps<Mention, Mention>, element?: HTMLElement) => void; key: (event: KeyboardEvent) => boolean; hide: () => void }

// mentions links a record where @ is typed: find searches by title, or a
// page by its path, and the bridge shows the matches.
function mentions(find: (query: string) => Promise<Mention[]>, bridge: Bridge) {
  return Extension.create({
    name: 'mentions',
    addProseMirrorPlugins() {
      return [
        Suggestion<Mention, Mention>({
          editor: this.editor,
          allowSpaces: true,
          debounce: 120,
          items: ({ query }) => find(query),
          command: ({ editor, range, props }) => {
            editor
              .chain()
              .focus()
              .insertContentAt(range, [
                { type: 'text', text: props.name, marks: [{ type: 'link', attrs: { href: `/r/${props.id}` } }] },
                { type: 'text', text: ' ' },
              ])
              .run()
          },
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

// matching finds people, companies and table records by title, and pages by
// title or by path: research/dan finds a page titled Dan… inside Research.
async function matching(query: string, objects: CrmObject[], pages: Pages | undefined): Promise<Mention[]> {
  const parts = query.toLowerCase().split('/').map((part) => part.trim())
  const title = parts.at(-1)!
  const named = (name: string) => name.toLowerCase().includes(title)
  const found = [...(pages?.byId.values() ?? [])]
    .map((page) => ({ page, trail: path(pages!, page.id).map((p) => p.name) }))
    .filter(({ trail }) => named(trail.at(-1) ?? '') && alongPath(parts.slice(0, -1), trail.slice(0, -1)))
    .slice(0, 5)
    .map(({ page, trail }) => ({ id: page.id, name: page.name, object: 'pages', icon: page.icon, detail: trail.slice(0, -1).join(' / ') || 'Page' }))
  const searched = parts.length > 1 ? [] : objects.filter((o) => o.slug === 'people' || o.slug === 'companies' || !o.standard)
  const results = await Promise.all(searched.map((o) => call<{ records: CrmRecord[] }>('search_records', { object: o.slug, query: title, limit: 8 })))
  const records = results.flatMap((result, i) =>
    result.records.filter((r) => named(recordName(r))).map((r) => ({ id: r.id, name: recordName(r), object: r.object, detail: singular(searched[i]), photo: r.photo })),
  )
  const crm = (m: Mention) => m.object === 'people' || m.object === 'companies'
  return [...records.filter(crm), ...found, ...records.filter((m) => !crm(m))].slice(0, 12)
}

// alongPath reports whether each part names one of the ancestors, in order.
function alongPath(parts: string[], ancestors: string[]) {
  let matched = 0
  for (const name of ancestors) {
    if (matched < parts.length && name.toLowerCase().includes(parts[matched])) {
      matched++
    }
  }
  return matched === parts.length
}

// MarkdownView shows short markdown, such as a person's context, read-only
// with the page typography at a compact size.
export function MarkdownView({ text }: { text: string }) {
  const editor = useEditor({
    editable: false,
    // An editor keeps an empty line after a final list for typing into; a
    // view would add one on every click.
    extensions: [DocumentKit.configure({ trailingNode: false }), TaskList, TaskItem, TableKit, Markdown],
    content: text,
    contentType: 'markdown',
    editorProps: { attributes: { class: 'prose-page prose-compact' } },
  }, [text])
  return <EditorContent editor={editor} />
}

// PageEditor edits a page's markdown content in place. It saves shortly after
// typing stops, on blur and on leaving, each save expecting the content it
// last saw, so an agent's edit is never overwritten unseen: a conflicting
// save shows the latest content instead.
export function PageEditor({ record }: { record: CrmRecord }) {
  const saved = valuesOf(record, 'content').map(valueText)[0] ?? ''
  const client = useQueryClient()
  const navigate = useNavigate()
  const objects = useObjects()
  const pages = usePages()
  const sources = useRef({ objects, pages })
  useEffect(() => {
    sources.current = { objects, pages }
  })
  const [find] = useState(() => (query: string) => matching(query, sources.current.objects ?? [], sources.current.pages))
  // base is the content the server holds as far as this editor knows;
  // latest is typed content not yet saved.
  const base = useRef(saved)
  const latest = useRef<string | null>(null)
  const timer = useRef(0)
  const saving = useRef<Promise<void> | null>(null)
  const [menu, setMenu] = useState<Menu | null>(null)
  const current = useRef<Menu | null>(null)
  const [bridge] = useState<Bridge>(() => {
    const set = (next: Menu | null) => {
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

  const view = useRef<Editor | null>(null)
  const [save] = useState(() => async (): Promise<void> => {
    window.clearTimeout(timer.current)
    if (saving.current) {
      await saving.current
    }
    const text = latest.current
    latest.current = null
    if (text === null || text === base.current) {
      return
    }
    const write = text ? { values: { content: text } } : { remove: { content: [] } }
    saving.current = call('upsert_record', { object: record.object, record_id: record.id, expect: { content: base.current }, ...write })
      .then(() => {
        base.current = text
        void client.invalidateQueries({ queryKey: toolQuery('get_record', { record_id: record.id }).queryKey })
      })
      .catch(async (error: Error) => {
        toast.error(`Not saved: ${error.message}. Showing the latest version.`)
        const fresh = await client.fetchQuery({ ...toolQuery<CrmRecord>('get_record', { record_id: record.id }), staleTime: 0 })
        base.current = valuesOf(fresh, 'content').map(valueText)[0] ?? ''
        latest.current = null
        view.current?.commands.setContent(base.current, { contentType: 'markdown', emitUpdate: false })
      })
      .finally(() => {
        saving.current = null
      })
    await saving.current
  })

  const editor = useEditor({
    extensions: [
      DocumentKit.configure({ link: { openOnClick: false } }),
      TaskList,
      TaskItem.configure({ nested: true }),
      TableKit,
      Placeholder.configure({ placeholder: 'Write, or type @ to mention' }),
      Markdown,
      mentions(find, bridge),
    ],
    content: saved,
    contentType: 'markdown',
    editorProps: {
      attributes: { class: 'prose-page min-h-20 outline-none', 'aria-label': 'Content' },
      handleClick: (_view, _pos, event) => {
        const href = (event.target as HTMLElement).closest('a')?.getAttribute('href')
        if (!href) {
          return false
        }
        event.preventDefault()
        const recordId = href.match(/^\/r\/([^/?#]+)$/)?.[1]
        if (recordId) {
          void navigate({ to: '/r/$recordId', params: { recordId } })
        } else if (embedded()) {
          void app.openLink({ url: href })
        } else {
          window.open(href, '_blank', 'noopener,noreferrer')
        }
        return true
      },
    },
    onUpdate: ({ editor }) => {
      latest.current = editor.getMarkdown().trim()
      window.clearTimeout(timer.current)
      timer.current = window.setTimeout(() => void save(), 600)
    },
    onBlur: () => void save(),
  })

  // Content someone else saved replaces what is shown, unless typing is
  // still unsaved; that save then finds the conflict.
  useEffect(() => {
    view.current = editor
    if (editor && saved !== base.current && latest.current === null && !saving.current) {
      base.current = saved
      editor.commands.setContent(saved, { contentType: 'markdown', emitUpdate: false })
    }
  }, [editor, saved])
  useEffect(() => () => void save(), [save])

  return (
    <>
      <EditorContent editor={editor} />
      {menu &&
        createPortal(
          <div role="listbox" aria-label="Mention" className="w-80 overflow-hidden rounded-[var(--radius-card)] bg-raised p-1.5 text-[13px] shadow-[var(--shadow-raised)]">
            {menu.items.length === 0 && <div className="px-2 py-2 text-[12px] text-ink-3">{menu.loading ? 'Searching…' : 'No matches'}</div>}
            {menu.items.map((item, index) => (
              <button
                key={item.id}
                type="button"
                role="option"
                aria-selected={index === menu.index}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => menu.pick(item)}
                className={cn('flex h-8 w-full items-center gap-2.5 rounded-[var(--radius-control)] px-2 text-left text-ink outline-none', index === menu.index && 'bg-list-active')}
              >
                <RecordIcon object={item.object} name={item.name} photo={item.photo} icon={item.icon} size={16} />
                <span className="min-w-0 flex-1 truncate">{item.name}</span>
                <span className="max-w-36 shrink-0 truncate text-[12px] text-ink-3">{item.detail}</span>
              </button>
            ))}
          </div>,
          menu.element,
        )}
    </>
  )
}
