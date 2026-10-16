import type { Editor } from '@tiptap/core'
import { BetweenHorizontalEnd, BetweenHorizontalStart, BetweenVerticalEnd, BetweenVerticalStart, ChevronDown, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu'

const actions = [
  { name: 'Insert row above', icon: BetweenHorizontalStart, run: (editor: Editor) => editor.chain().focus().addRowBefore().run() },
  { name: 'Insert row below', icon: BetweenHorizontalEnd, run: (editor: Editor) => editor.chain().focus().addRowAfter().run() },
  { name: 'Insert column left', icon: BetweenVerticalStart, run: (editor: Editor) => editor.chain().focus().addColumnBefore().run() },
  { name: 'Insert column right', icon: BetweenVerticalEnd, run: (editor: Editor) => editor.chain().focus().addColumnAfter().run() },
]

// TableMenu edits the rows and columns around the table cell being typed in,
// from a handle on the cell's right edge.
export function TableMenu({ editor, frame }: { editor: Editor; frame: HTMLElement }) {
  const [cell, setCell] = useState<{ top: number; left: number } | null>(null)
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (open) {
      return
    }
    const update = () => {
      const { $from } = editor.state.selection
      let depth = $from.depth
      while (depth > 0 && !String($from.node(depth).type.spec.tableRole).endsWith('cell')) {
        depth--
      }
      const dom = depth > 0 && editor.isEditable && editor.view.hasFocus() && editor.view.nodeDOM($from.before(depth))
      if (!(dom instanceof HTMLElement)) {
        setCell(null)
        return
      }
      const box = dom.getBoundingClientRect()
      const origin = frame.getBoundingClientRect()
      setCell({ top: box.top - origin.top + box.height / 2, left: box.right - origin.left })
    }
    editor.on('selectionUpdate', update)
    editor.on('update', update)
    editor.on('focus', update)
    editor.on('blur', update)
    return () => {
      editor.off('selectionUpdate', update)
      editor.off('update', update)
      editor.off('focus', update)
      editor.off('blur', update)
    }
  }, [editor, frame, open])

  if (!cell) {
    return null
  }
  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger aria-label="Table options" title="Table options" style={cell} className="absolute z-10 grid h-5 w-4 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-[4px] bg-raised text-ink-2 shadow-[var(--shadow-raised)] outline-none hover:text-ink focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:text-ink pointer-coarse:h-7 pointer-coarse:w-6">
        <ChevronDown className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-52" onCloseAutoFocus={(event) => {
        event.preventDefault()
        editor.commands.focus()
      }}>
        {actions.map((action) => <DropdownMenuItem key={action.name} onSelect={() => action.run(editor)}><action.icon /> {action.name}</DropdownMenuItem>)}
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => editor.chain().focus().deleteRow().run()}><Trash2 /> Delete row</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => editor.chain().focus().deleteColumn().run()}><Trash2 /> Delete column</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
