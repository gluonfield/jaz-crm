import { Button } from '@jaz/ui/button'
import type { Editor } from '@tiptap/core'
import { NodeSelection, Selection, type Transaction } from '@tiptap/pm/state'
import { Check, Copy, GripVertical, Palette, Plus, Repeat2, Trash2 } from 'lucide-react'
import { type DragEvent, useEffect, useState } from 'react'
import { colorValue, iconColors } from '@/lib/page-icons'
import { addBelow, blockAt, blocks, deleteBlock, duplicateBlock, kindOf, turnBlock } from './editor-blocks'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuSub, DropdownMenuSubContent, DropdownMenuSubTrigger, DropdownMenuTrigger } from './ui/dropdown-menu'

type Spot = { pos: number; top: number; left: number }

// A selected block is replaced by the next key typed, so once a block is
// dropped or its menu closes the caret returns to text.
const release = (editor: Editor) => editor.chain().focus().command(({ tr }) => {
  if (tr.selection instanceof NodeSelection) {
    const $end = tr.doc.resolve(tr.selection.node.isAtom ? tr.selection.to : tr.selection.to - 1)
    const text = Selection.findFrom($end, -1, true) ?? Selection.findFrom($end, 1, true)
    if (text) {
      tr.setSelection(text)
    }
  }
  return true
}).run()

// drag hands a block to ProseMirror's drag and drop, which moves it to where
// it is dropped.
function drag(editor: Editor, pos: number, event: DragEvent) {
  editor.commands.setNodeSelection(pos)
  const { view } = editor
  const slice = view.state.selection.content()
  const { dom, text } = view.serializeForClipboard(slice)
  event.dataTransfer.clearData()
  event.dataTransfer.setData('text/html', dom.innerHTML)
  event.dataTransfer.setData('text/plain', text)
  event.dataTransfer.effectAllowed = 'copyMove'
  event.dataTransfer.setDragImage(view.nodeDOM(pos) as HTMLElement, 0, 0)
  view.dragging = { slice, move: true }
}

// A drag dropped outside the editor must not leave a move pending.
const endDrag = (editor: Editor) => {
  editor.view.dragging = null
}

// BlockHandle puts Notion's + and drag handle beside the block under the
// pointer, within frame: + starts a line below the block, dragging moves it
// and a click opens its menu. Typing hides them.
export function BlockHandle({ editor, frame }: { editor: Editor; frame: HTMLElement }) {
  const [spot, setSpot] = useState<Spot | null>(null)
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (open) {
      return
    }
    const { view } = editor
    const place = (pos: number): Spot | null => {
      const dom = view.nodeDOM(pos)
      if (!(dom instanceof HTMLElement)) {
        return null
      }
      const box = dom.getBoundingClientRect()
      const style = getComputedStyle(dom)
      const pad = parseFloat(style.paddingTop)
      const line = Math.min(parseFloat(style.lineHeight) || box.height, box.height - pad)
      // A bullet or number sits in its list's padding, left of the item.
      const marker = dom.parentElement?.matches('ul:not([data-type=taskList]), ol') ? parseFloat(getComputedStyle(dom.parentElement).paddingLeft) : 0
      const origin = frame.getBoundingClientRect()
      return { pos, top: box.top - origin.top + pad + line / 2, left: box.left - origin.left - marker }
    }
    const move = (event: MouseEvent) => {
      const box = view.dom.getBoundingClientRect()
      const hit = editor.isEditable && view.posAtCoords({ left: box.right - 2, top: Math.min(Math.max(event.clientY, box.top + 1), box.bottom - 1) })
      setSpot(hit ? place(blockAt(view.state.doc, hit.inside < 0 ? hit.pos : hit.inside)) : null)
    }
    const hide = () => setSpot(null)
    const typed = ({ transaction }: { transaction: Transaction }) => {
      if (transaction.getMeta('uiEvent') === 'drop') {
        queueMicrotask(() => release(editor))
      }
      if (transaction.docChanged) {
        hide()
      }
    }
    frame.addEventListener('mousemove', move)
    frame.addEventListener('mouseleave', hide)
    editor.on('transaction', typed)
    return () => {
      frame.removeEventListener('mousemove', move)
      frame.removeEventListener('mouseleave', hide)
      editor.off('transaction', typed)
    }
  }, [editor, frame, open])

  if (!spot || !editor.state.doc.nodeAt(spot.pos)) {
    return null
  }
  const kind = kindOf(editor.state.doc, spot.pos)
  const { color } = editor.state.doc.nodeAt(spot.pos)!.attrs
  return (
    <div className="absolute z-10 flex -translate-x-full -translate-y-1/2 items-center pr-1 pointer-coarse:hidden" style={{ top: spot.top, left: spot.left }}>
      <Button variant="ghost" size="icon-sm" aria-label="Add below" title="Add below" onMouseDown={(e) => e.preventDefault()} onClick={() => {
        addBelow(editor, spot.pos)
        editor.commands.focus()
      }}><Plus /></Button>
      <div className="relative">
        <Button variant="ghost" size="icon-sm" draggable aria-label="Drag to move, click for options" title="Drag to move, click for options" className="w-5 cursor-grab active:cursor-grabbing" onClick={() => {
          editor.commands.setNodeSelection(spot.pos)
          setOpen(true)
        }} onDragStart={(event) => drag(editor, spot.pos, event)} onDragEnd={() => endDrag(editor)}><GripVertical /></Button>
        {/* The menu opens on click, not on the trigger's pointer down, so
            dragging the handle never opens it; the trigger only anchors it. */}
        <DropdownMenu open={open} onOpenChange={setOpen}>
          <DropdownMenuTrigger asChild><span aria-hidden className="pointer-events-none absolute bottom-0 left-0" /></DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-52" onCloseAutoFocus={(event) => {
            event.preventDefault()
            release(editor)
          }}>
            {kind && <DropdownMenuSub>
              <DropdownMenuSubTrigger><Repeat2 /> Turn into</DropdownMenuSubTrigger>
              <DropdownMenuSubContent className="w-48">
                {blocks.filter((block) => block.is).map((block) => (
                  <DropdownMenuItem key={block.name} onSelect={() => turnBlock(editor, spot.pos, block)}>
                    <block.icon />
                    <span className="min-w-0 flex-1 truncate">{block.name}</span>
                    {block === kind && <Check className="text-ink" />}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuSubContent>
            </DropdownMenuSub>}
            {color !== undefined && <DropdownMenuSub>
              <DropdownMenuSubTrigger><Palette /> Color</DropdownMenuSubTrigger>
              <DropdownMenuSubContent className="w-44">
                {iconColors.map((choice) => (
                  <DropdownMenuItem key={choice} onSelect={() => editor.commands.command(({ tr }) => {
                    tr.setNodeAttribute(spot.pos, 'color', choice)
                    return true
                  })}>
                    <span className="size-4 rounded-[4px]" style={{ background: `color-mix(in oklab, ${colorValue(choice)} 25%, transparent)` }} />
                    <span className="min-w-0 flex-1 truncate capitalize">{choice}</span>
                    {choice === color && <Check className="text-ink" />}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuSubContent>
            </DropdownMenuSub>}
            <DropdownMenuItem onSelect={() => duplicateBlock(editor, spot.pos)}><Copy /> Duplicate</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => deleteBlock(editor, spot.pos)}><Trash2 /> Delete</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  )
}
