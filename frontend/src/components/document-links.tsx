import { type Editor, getMarkRange, type MarkViewRenderer } from '@tiptap/core'
import { TaskItem, TaskList } from '@tiptap/extension-list'
import { TableKit } from '@tiptap/extension-table'
import { Markdown } from '@tiptap/markdown'
import { TextSelection } from '@tiptap/pm/state'
import { ReactRenderer } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import { Link2 } from 'lucide-react'
import { recordName } from '@/lib/crm'
import { pageIcon } from '@/lib/pages'
import { useTool } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { RecordIcon } from './icons'

export function documentLinkAt(editor: Editor, position: number) {
  const doc = editor.state.doc
  const range = getMarkRange(doc.resolve(position), editor.schema.marks.link)
  const mark = range && doc.nodeAt(range.from)?.marks.find((mark) => mark.type.name === 'link')
  return range && mark ? { ...range, doc, mark, text: doc.textBetween(range.from, range.to) } : null
}

export function editDocumentLink(editor: Editor, link: NonNullable<ReturnType<typeof documentLinkAt>>, text: string, href: string) {
  if (!editor.isEditable || editor.state.doc !== link.doc || !text || !href || !editor.can().setLink({ href })) {
    return false
  }
  const tr = editor.state.tr
  if (text !== link.text) {
    tr.replaceWith(link.from, link.to, editor.schema.text(text, link.doc.nodeAt(link.from)!.marks))
  }
  const end = link.from + text.length
  tr.removeMark(link.from, end, link.mark.type)
  tr.addMark(link.from, end, link.mark.type.create({ ...link.mark.attrs, href }))
  tr.setSelection(TextSelection.create(tr.doc, end))
  tr.setMeta('preventAutolink', true)
  editor.view.dispatch(tr)
  return true
}

function DocumentLinkIcon({ href }: { href: string }) {
  const id = href.match(/^\/r\/([^/?#]+)$/)?.[1]
  const { data: record } = useTool<CrmRecord>('get_record', { record_id: id }, { enabled: !!id, staleTime: 60_000, retry: false })
  return id ? record && <RecordIcon object={record.object} name={recordName(record)} photo={record.photo} icon={pageIcon(record)} size={14} /> : <Link2 size={14} />
}

// ProseMirror owns the text DOM synchronously; React only owns the icon.
const documentLink: MarkViewRenderer = ({ editor, mark, HTMLAttributes }) => {
  const dom = document.createElement('a')
  for (const [name, value] of Object.entries(HTMLAttributes)) {
    if (value !== null && value !== undefined) {
      dom.setAttribute(name, String(value))
    }
  }
  if (!editor.can().setLink({ href: mark.attrs.href })) {
    dom.removeAttribute('href')
  }
  const icon = new ReactRenderer(DocumentLinkIcon, { editor, props: { href: mark.attrs.href }, as: 'span', className: 'record-link-icon' })
  icon.element.contentEditable = 'false'
  icon.element.setAttribute('aria-hidden', 'true')
  const contentDOM = document.createElement('span')
  dom.append(icon.element, contentDOM)
  return {
    dom,
    contentDOM,
    ignoreMutation: (mutation) => mutation.type !== 'selection' && icon.element.contains(mutation.target),
    destroy: () => icon.destroy(),
  }
}

export const DocumentKit = StarterKit.extend({
  addExtensions() {
    return [
      ...this.parent!().map((extension) => extension.name === 'link' ? extension.extend({ addMarkView: () => documentLink }) : extension),
      TaskList,
      TaskItem.configure({ nested: true }),
      TableKit,
      Markdown,
    ]
  },
})
