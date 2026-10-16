import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import type { Editor } from '@tiptap/core'
import { Placeholder } from '@tiptap/extensions'
import { cellAround } from '@tiptap/pm/tables'
import { EditorContent, useEditor } from '@tiptap/react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { call, embedded } from '@/lib/api'
import { recordName, valueText, valuesOf } from '@/lib/crm'
import { app } from '@/lib/mcp-app'
import { type Pages, path, usePages } from '@/lib/pages'
import { toolQuery, useObjects } from '@/lib/queries'
import type { CrmObject, CrmRecord } from '@/lib/types'
import { BlockHandle } from './block-handle'
import { singular } from './create-record'
import { DocumentLinkEditor } from './document-link-editor'
import { DocumentKit } from './document-links'
import { type Block, findBlocks } from './editor-blocks'
import { suggestions } from './editor-suggestions'
import { FormatBar } from './format-bar'
import { ObjectIcon, RecordIcon } from './icons'
import { TableMenu } from './table-menu'
import { MenuShortcut } from './ui/menu'

type Mention = { id: string; href: string; name: string; object: string; detail: string; photo?: string; icon?: string }

// Markdown table cells hold only text, so / turns or adds blocks outside them.
const blockMenu = suggestions<Block>({
  char: '/',
  find: findBlocks,
  allow: (state, range) => !cellAround(state.doc.resolve(range.from)),
  pick: (editor, range, block) => block.run(editor.chain().focus().deleteRange(range)).run(),
  label: 'Blocks',
  empty: 'No results',
  className: 'w-60',
  item: (block) => <>
    <block.icon className="size-4 shrink-0 text-ink-2" />
    <span className="min-w-0 flex-1 truncate">{block.name}</span>
    {block.hint && <MenuShortcut>{block.hint}</MenuShortcut>}
  </>,
})

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
    .map(({ page, trail }) => ({ id: page.id, href: `/r/${page.id}`, name: page.name, object: 'pages', icon: page.icon, detail: trail.slice(0, -1).join(' / ') || 'Page' }))
  const tables = objects.filter((table) => !table.standard && named(table.name))
    .filter((table) => alongPath(parts.slice(0, -1), table.parent && pages ? path(pages, table.parent).map((page) => page.name) : []))
    .map((table) => ({ id: `/o/${table.slug}`, href: `/o/${table.slug}`, name: table.name, object: table.slug, icon: table.icon, detail: 'Database' }))
  const searched = parts.length > 1 ? [] : objects.filter((o) => o.slug === 'people' || o.slug === 'companies' || !o.standard)
  const results = await Promise.all(searched.map((o) => call<{ records: CrmRecord[] }>('search_records', { object: o.slug, query: title, limit: 8 })))
  const records = results.flatMap((result, i) =>
    result.records.filter((r) => named(recordName(r))).map((r) => ({ id: r.id, href: `/r/${r.id}`, name: recordName(r), object: r.object, detail: singular(searched[i]), photo: r.photo })),
  )
  const crm = (m: Mention) => m.object === 'people' || m.object === 'companies'
  return [...records.filter(crm), ...found, ...tables, ...records.filter((m) => !crm(m))].slice(0, 12)
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
    extensions: [DocumentKit.configure({ trailingNode: false })],
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
  const [frame, setFrame] = useState<HTMLDivElement | null>(null)

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
      DocumentKit.configure({ link: { openOnClick: false }, dropcursor: { color: 'var(--color-primary)', width: 2 } }),
      Placeholder.configure({
        includeChildren: true,
        placeholder: ({ node }) => node.type.name === 'heading' ? `Heading ${node.attrs.level}` : "Type '/' for commands, '@' to mention",
      }),
      // A mention links a record where @ is typed: find searches by title,
      // or a page by its path.
      suggestions<Mention>({
        char: '@',
        find,
        allowSpaces: true,
        debounce: 120,
        pick: (editor, range, item) => editor.chain().focus().insertContentAt(range, [
          { type: 'text', text: item.name, marks: [{ type: 'link', attrs: { href: item.href } }] },
          { type: 'text', text: ' ' },
        ]).run(),
        label: 'Mention',
        empty: 'No matches',
        className: 'w-80',
        item: (item) => <>
          {item.href.startsWith('/o/') ? <ObjectIcon slug={item.object} icon={item.icon} className="size-4 shrink-0 text-ink-2" /> : <RecordIcon object={item.object} name={item.name} photo={item.photo} icon={item.icon} size={16} />}
          <span className="min-w-0 flex-1 truncate">{item.name}</span>
          <span className="max-w-36 shrink-0 truncate text-[12px] text-ink-3">{item.detail}</span>
        </>,
      }),
      blockMenu,
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
        if (/^\/(r|o)\/[^/?#]+$/.test(href)) {
          void navigate({ href })
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
    <div ref={setFrame} className="relative -ml-14 pl-14">
      <EditorContent editor={editor} />
      {editor && <DocumentLinkEditor editor={editor} />}
      {editor && <FormatBar editor={editor} />}
      {editor && frame && <BlockHandle editor={editor} frame={frame} />}
      {editor && frame && <TableMenu editor={editor} frame={frame} />}
    </div>
  )
}
