import type { ChainedCommands, Editor } from '@tiptap/core'
import type { Node } from '@tiptap/pm/model'
import { TextSelection } from '@tiptap/pm/state'
import { CodeXml, Heading1, Heading2, Heading3, List, ListOrdered, ListTodo, type LucideIcon, Minus, Pilcrow, Quote, SquareMenu, Table } from 'lucide-react'

// A Block is a kind the / menu creates. Blocks with is are kinds a block can
// be turned into; the rest insert a new block.
export type Block = {
  name: string
  icon: LucideIcon
  hint?: string
  keywords: string
  run: (chain: ChainedCommands) => ChainedCommands
  is?: (node: Node, parent: Node) => boolean
}

const heading = (level: 1 | 2 | 3, icon: LucideIcon): Block => ({
  name: `Heading ${level}`,
  icon,
  hint: '#'.repeat(level),
  keywords: `h${level} title`,
  run: (chain) => chain.setHeading({ level }),
  is: (node) => node.type.name === 'heading' && node.attrs.level === level,
})

export const blocks: Block[] = [
  { name: 'Text', icon: Pilcrow, keywords: 'paragraph plain', run: (chain) => chain.clearNodes(), is: (node) => node.type.name === 'paragraph' },
  heading(1, Heading1),
  heading(2, Heading2),
  heading(3, Heading3),
  { name: 'Bulleted list', icon: List, hint: '-', keywords: 'bullet unordered', run: (chain) => chain.clearNodes().toggleBulletList(), is: (node, parent) => node.type.name === 'listItem' && parent.type.name === 'bulletList' },
  { name: 'Numbered list', icon: ListOrdered, hint: '1.', keywords: 'ordered', run: (chain) => chain.clearNodes().toggleOrderedList(), is: (node, parent) => node.type.name === 'listItem' && parent.type.name === 'orderedList' },
  { name: 'Checklist', icon: ListTodo, hint: '[]', keywords: 'todo to-do task checkbox', run: (chain) => chain.clearNodes().toggleTaskList(), is: (node) => node.type.name === 'taskItem' },
  { name: 'Callout', icon: SquareMenu, keywords: 'note tip warning aside', run: (chain) => chain.clearNodes().wrapIn('callout'), is: (node) => node.type.name === 'callout' },
  { name: 'Quote', icon: Quote, hint: '>', keywords: 'blockquote', run: (chain) => chain.clearNodes().setBlockquote(), is: (node) => node.type.name === 'blockquote' },
  { name: 'Code', icon: CodeXml, hint: '```', keywords: 'codeblock snippet', run: (chain) => chain.setCodeBlock(), is: (node) => node.type.name === 'codeBlock' },
  { name: 'Table', icon: Table, keywords: 'grid', run: (chain) => chain.insertTable({ rows: 3, cols: 3, withHeaderRow: true }) },
  { name: 'Divider', icon: Minus, hint: '---', keywords: 'horizontal rule separator line', run: (chain) => chain.setHorizontalRule() },
]

export const findBlocks = (query: string) => blocks.filter((block) => `${block.name} ${block.keywords}`.toLowerCase().includes(query.toLowerCase()))

const items = new Set(['listItem', 'taskItem'])

// blockAt finds the block a handle acts on for the node at inside: the list
// item holding it, else its top-level node.
export function blockAt(doc: Node, inside: number) {
  if (items.has(doc.nodeAt(inside)?.type.name ?? '')) {
    return inside
  }
  const $pos = doc.resolve(inside)
  for (let depth = $pos.depth; depth > 0; depth--) {
    if (items.has($pos.node(depth).type.name)) {
      return $pos.before(depth)
    }
  }
  return $pos.depth ? $pos.before(1) : inside
}

export const kindOf = (doc: Node, pos: number) => blocks.find((block) => block.is?.(doc.nodeAt(pos)!, doc.resolve(pos).parent))

export function turnBlock(editor: Editor, pos: number, block: Block) {
  return block.run(editor.chain().command(({ tr }) => {
    tr.setSelection(TextSelection.near(tr.doc.resolve(pos + 1)))
    return true
  })).run()
}

export function duplicateBlock(editor: Editor, pos: number) {
  const node = editor.state.doc.nodeAt(pos)!
  editor.view.dispatch(editor.state.tr.insert(pos + node.nodeSize, node))
}

export const deleteBlock = (editor: Editor, pos: number) => editor.chain().setNodeSelection(pos).deleteSelection().run()

// addBelow starts a line under the block with the / menu open, as Notion's +
// does: a list item gets a sibling item and an empty line is used as it is.
export function addBelow(editor: Editor, pos: number) {
  const node = editor.state.doc.nodeAt(pos)!
  if (node.type.name === 'paragraph' && !node.content.size) {
    return editor.commands.insertContentAt(pos + 1, '/')
  }
  const line = { type: 'paragraph', content: [{ type: 'text', text: '/' }] }
  return editor.commands.insertContentAt(pos + node.nodeSize, items.has(node.type.name) ? { type: node.type.name, content: [line] } : line)
}
