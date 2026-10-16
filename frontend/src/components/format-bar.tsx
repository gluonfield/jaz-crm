import { Button } from '@jaz/ui/button'
import type { Editor } from '@tiptap/core'
import { TextSelection } from '@tiptap/pm/state'
import { useEditorState } from '@tiptap/react'
import { BubbleMenu } from '@tiptap/react/menus'
import { Bold, Code, Italic, Link2, type LucideIcon, Strikethrough } from 'lucide-react'
import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'
import { inputClass } from './controls'

type Look = { label: string; keys: string; icon: LucideIcon }

const marks = [
  { name: 'bold', label: 'Bold', keys: '⌘B', icon: Bold },
  { name: 'italic', label: 'Italic', keys: '⌘I', icon: Italic },
  { name: 'strike', label: 'Strikethrough', keys: '⌘⇧S', icon: Strikethrough },
  { name: 'code', label: 'Code', keys: '⌘E', icon: Code },
]

const link: Look = { label: 'Link', keys: '⌘K', icon: Link2 }

function Mark({ mark: { label, keys, icon: Icon }, on, onClick }: { mark: Look; on: boolean; onClick: () => void }) {
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label={label}
      title={`${label} ${keys}`}
      aria-pressed={on}
      className={cn(on && 'bg-list-active text-ink hover:bg-list-active')}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
    >
      <Icon />
    </Button>
  )
}

// FormatBar formats selected text, as Notion's selection toolbar does. ⌘K
// on a selection without a link asks for one.
export function FormatBar({ editor }: { editor: Editor }) {
  const [linking, setLinking] = useState(false)
  const active = useEditorState({ editor, selector: ({ editor }) => ({ marks: marks.map((mark) => editor.isActive(mark.name)), link: editor.isActive('link') }) })

  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      const { selection } = editor.state
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey) && !selection.empty && !editor.isActive('link')) {
        event.preventDefault()
        setLinking(true)
      }
    }
    editor.view.dom.addEventListener('keydown', key)
    return () => editor.view.dom.removeEventListener('keydown', key)
  }, [editor])

  return (
    <BubbleMenu
      editor={editor}
      shouldShow={({ editor, element, state }) => editor.isEditable && state.selection instanceof TextSelection && !state.selection.empty && !editor.isActive('codeBlock') && (editor.view.hasFocus() || element.contains(document.activeElement))}
      options={{ placement: 'top-start', offset: 8, onHide: () => setLinking(false) }}
      className="z-50 flex items-center gap-0.5 rounded-[var(--radius-card)] border bg-raised p-1 shadow-[var(--shadow-raised)]"
    >
      {linking ? (
        <form className="p-0.5" onSubmit={(event) => {
          event.preventDefault()
          const url = event.currentTarget.elements.namedItem('url') as HTMLInputElement
          if (!editor.chain().focus().setLink({ href: url.value.trim() }).run()) {
            url.setCustomValidity('Use a valid link destination.')
            url.reportValidity()
            return
          }
          setLinking(false)
        }}>
          <input
            name="url"
            required
            autoFocus
            aria-label="Link"
            placeholder="Paste a link"
            onInput={(event) => event.currentTarget.setCustomValidity('')}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                setLinking(false)
                editor.commands.focus()
              }
            }}
            className={`${inputClass} w-64`}
          />
        </form>
      ) : <>
        {marks.map((mark, index) => <Mark key={mark.name} mark={mark} on={active.marks[index]} onClick={() => editor.chain().focus().toggleMark(mark.name).run()} />)}
        <Mark mark={link} on={active.link} onClick={() => {
          if (active.link) {
            editor.chain().focus().extendMarkRange('link').unsetLink().run()
          } else {
            setLinking(true)
          }
        }} />
      </>}
    </BubbleMenu>
  )
}
