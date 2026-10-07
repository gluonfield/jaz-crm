import type { MarkViewProps } from '@tiptap/core'
import { MarkViewContent, ReactMarkViewRenderer } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import { recordName } from '@/lib/crm'
import { pageIcon } from '@/lib/pages'
import { useTool } from '@/lib/queries'
import type { CrmRecord } from '@/lib/types'
import { RecordIcon } from './icons'

function DocumentLink({ mark, HTMLAttributes }: MarkViewProps) {
  const id = String(mark.attrs.href).match(/^\/r\/([^/?#]+)$/)?.[1]
  const { data: record } = useTool<CrmRecord>('get_record', { record_id: id }, { enabled: !!id, staleTime: 60_000, retry: false })
  return (
    <a {...HTMLAttributes}>
      {id && <span contentEditable={false} aria-hidden className="record-link-icon">{record && <RecordIcon object={record.object} name={recordName(record)} photo={record.photo} icon={pageIcon(record)} size={14} />}</span>}
      <MarkViewContent />
    </a>
  )
}

export const DocumentKit = StarterKit.extend({
  addExtensions() {
    return this.parent!().map((extension) => extension.name === 'link' ? extension.extend({ addMarkView: () => ReactMarkViewRenderer(DocumentLink) }) : extension)
  },
})
