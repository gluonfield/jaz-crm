import { Button } from '@jaz/ui/button'
import type { Editor } from '@tiptap/core'
import type { Transaction } from '@tiptap/pm/state'
import { Pencil } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { inputClass } from './controls'
import { documentLinkAt, editDocumentLink } from './document-links'
import { Popover, PopoverAnchor, PopoverContent } from './ui/popover'

type Link = NonNullable<ReturnType<typeof documentLinkAt>> & { anchor: HTMLAnchorElement; editing: boolean }

export function DocumentLinkEditor({ editor }: { editor: Editor }) {
  const [link, setLink] = useState<Link | null>(null)
  const form = useRef<HTMLFormElement>(null)
  const timer = useRef(0)
  const keepOpen = useCallback(() => window.clearTimeout(timer.current), [])
  const leave = useCallback(() => {
    keepOpen()
    timer.current = window.setTimeout(() => {
      if (!form.current?.contains(document.activeElement)) {
        setLink(null)
      }
    }, 200)
  }, [keepOpen])

  useEffect(() => {
    if (link?.editing) {
      form.current?.querySelector('input')?.focus()
    }
  }, [link])

  useEffect(() => {
    const dom = editor.view.dom
    const show = (anchor: HTMLAnchorElement, editing = false) => {
      window.clearTimeout(timer.current)
      const target = documentLinkAt(editor, editor.view.posAtDOM(anchor, 0))
      if (target) {
        setLink((current) => current?.anchor === anchor && !editing ? current : { ...target, anchor, editing })
      }
    }
    const hover = (event: PointerEvent) => {
      const anchor = (event.target as Element).closest<HTMLAnchorElement>('a[href]')
      if (anchor && event.pointerType !== 'touch') {
        show(anchor)
      }
    }
    const key = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        const point = editor.view.domAtPos(editor.state.selection.from).node
        const anchor = (point instanceof Element ? point : point.parentElement)?.closest<HTMLAnchorElement>('a[href]')
        if (anchor) {
          event.preventDefault()
          show(anchor, true)
        }
      }
    }
    const change = ({ transaction }: { transaction: Transaction }) => {
      if (transaction.docChanged) {
        setLink(null)
      }
    }
    dom.addEventListener('pointerover', hover)
    dom.addEventListener('pointerout', leave)
    dom.addEventListener('keydown', key)
    editor.on('transaction', change)
    return () => {
      window.clearTimeout(timer.current)
      dom.removeEventListener('pointerover', hover)
      dom.removeEventListener('pointerout', leave)
      dom.removeEventListener('keydown', key)
      editor.off('transaction', change)
    }
  }, [editor, leave])

  return (
    <Popover open={!!link} onOpenChange={(open) => {
      if (!open) {
        setLink(null)
      }
    }}>
      <PopoverAnchor virtualRef={{ current: link?.anchor ?? null }} />
      {link && <PopoverContent aria-label="Edit link" align="start" className="w-80 max-w-[calc(100vw-24px)] p-2" onPointerEnter={keepOpen} onPointerLeave={leave} onOpenAutoFocus={(event) => event.preventDefault()} onCloseAutoFocus={(event) => event.preventDefault()} onEscapeKeyDown={() => editor.commands.focus()}>
        {link.editing ? <form key={`${link.from}:${link.mark.attrs.href}`} ref={form} className="space-y-2 p-1" onSubmit={(event) => {
          event.preventDefault()
          const fields = event.currentTarget.elements
          const text = (fields.namedItem('text') as HTMLInputElement).value.trim()
          const url = fields.namedItem('url') as HTMLInputElement
          if (editor.state.doc !== link.doc) {
            setLink(null)
            return
          }
          if (!editDocumentLink(editor, link, text, url.value.trim())) {
            url.setCustomValidity('Use a valid link destination.')
            url.reportValidity()
            return
          }
          setLink(null)
          editor.commands.focus()
        }}>
          <label className="block text-[12px] text-ink-2">Text<input name="text" required defaultValue={link.text} className={`${inputClass} mt-1 w-full`} /></label>
          <label className="block text-[12px] text-ink-2">URL<input name="url" required defaultValue={link.mark.attrs.href} onInput={(event) => event.currentTarget.setCustomValidity('')} className={`${inputClass} mt-1 w-full`} /></label>
          <div className="flex justify-end"><Button type="submit" variant="primary">Save</Button></div>
        </form> : <div className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate pl-1 text-[12px] text-ink-2" title={link.mark.attrs.href}>{link.mark.attrs.href}</span>
          <Button variant="ghost" size="icon" aria-label="Edit link" onClick={() => setLink({ ...link, editing: true })}><Pencil /></Button>
        </div>}
      </PopoverContent>}
    </Popover>
  )
}
