import type { ChainedCommands, Editor } from '@tiptap/core'
import { cellAround } from '@tiptap/pm/tables'
import { BetweenHorizontalEnd, BetweenHorizontalStart, BetweenVerticalEnd, BetweenVerticalStart, ChevronDown, type LucideIcon, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu'

type Action = { name: string; icon: LucideIcon; run: (chain: ChainedCommands) => ChainedCommands }

const inserts: Action[] = [
  { name: 'Insert row above', icon: BetweenHorizontalStart, run: (chain) => chain.addRowBefore() },
  { name: 'Insert row below', icon: BetweenHorizontalEnd, run: (chain) => chain.addRowAfter() },
  { name: 'Insert column left', icon: BetweenVerticalStart, run: (chain) => chain.addColumnBefore() },
  { name: 'Insert column right', icon: BetweenVerticalEnd, run: (chain) => chain.addColumnAfter() },
]

const removals: Action[] = [
  { name: 'Delete row', icon: Trash2, run: (chain) => chain.deleteRow() },
  { name: 'Delete column', icon: Trash2, run: (chain) => chain.deleteColumn() },
]

// TableMenu edits the rows and columns around the table cell being typed in,
// from a handle on the cell's right edge.
export function TableMenu({ editor, frame }: { editor: Editor; frame: HTMLElement }) {
  const [cell, setCell] = useState<{ top: number; left: number } | null>(null)
  const [open, setOpen] = useState(false)

  // Typing, moving the caret, focus and blur all arrive as transactions.
  useEffect(() => {
    if (open) {
      return
    }
    const update = () => {
      const $cell = cellAround(editor.state.selection.$head)
      const dom = $cell && editor.isEditable && editor.view.hasFocus() && editor.view.nodeDOM($cell.pos)
      if (!(dom instanceof HTMLElement)) {
        setCell(null)
        return
      }
      const box = dom.getBoundingClientRect()
      const origin = frame.getBoundingClientRect()
      setCell({ top: box.top - origin.top + box.height / 2, left: box.right - origin.left })
    }
    editor.on('transaction', update)
    return () => {
      editor.off('transaction', update)
    }
  }, [editor, frame, open])

  if (!cell) {
    return null
  }
  const item = (action: Action) => <DropdownMenuItem key={action.name} onSelect={() => action.run(editor.chain().focus()).run()}><action.icon /> {action.name}</DropdownMenuItem>
  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger aria-label="Table options" title="Table options" style={cell} className="absolute z-10 grid h-5 w-4 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-[4px] bg-raised text-ink-2 shadow-[var(--shadow-raised)] outline-none hover:text-ink focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:text-ink pointer-coarse:h-7 pointer-coarse:w-6">
        <ChevronDown className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-52" onCloseAutoFocus={(event) => {
        event.preventDefault()
        editor.commands.focus()
      }}>
        {inserts.map(item)}
        <DropdownMenuSeparator />
        {removals.map(item)}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
