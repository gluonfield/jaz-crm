import assert from 'node:assert/strict'
import { test } from 'node:test'
import { Editor } from '@tiptap/core'
import { DocumentKit } from './document-links'
import { addBelow, blockAt, blocks, deleteBlock, duplicateBlock, kindOf, turnBlock } from './editor-blocks'

const open = (markdown: string) => new Editor({ element: null, extensions: [DocumentKit], content: markdown, contentType: 'markdown' })

// saved returns the markdown a page stores, after checking it loads back to
// the same document.
function saved(editor: Editor) {
  const markdown = editor.getMarkdown()
  const loaded = open(markdown)
  assert.deepEqual(loaded.getJSON(), editor.getJSON())
  loaded.destroy()
  return markdown
}

// choose picks a block from the / menu just after /x was typed.
function choose(markdown: string, name: string) {
  const editor = open(markdown)
  let from = -1
  editor.state.doc.descendants((node, pos) => {
    if (node.isText && node.text!.includes('/x')) {
      from = pos + node.text!.indexOf('/x')
    }
  })
  editor.commands.setTextSelection(from + 2)
  const block = blocks.find((block) => block.name === name)!
  assert.equal(block.run(editor.chain().deleteRange({ from, to: from + 2 })).run(), true, name)
  const out = saved(editor)
  editor.destroy()
  return out
}

function textblock(editor: Editor, text: string) {
  let found = -1
  editor.state.doc.descendants((node, pos) => {
    if (node.isTextblock && node.textContent === text) {
      found = pos
    }
  })
  return found
}

test('the / menu turns the line it is typed on into each kind of block', () => {
  const line = 'Intro\n\n/xVisit Sheffield\n\nOutro'
  const turned: Record<string, string> = {
    Text: 'Visit Sheffield',
    'Heading 1': '# Visit Sheffield',
    'Heading 2': '## Visit Sheffield',
    'Heading 3': '### Visit Sheffield',
    'Bulleted list': '- Visit Sheffield',
    'Numbered list': '1. Visit Sheffield',
    Checklist: '- [ ] Visit Sheffield',
    Callout: '> [!callout] 💡\n> Visit Sheffield',
    Quote: '> Visit Sheffield',
    Code: '```\nVisit Sheffield\n```',
  }
  for (const [name, markdown] of Object.entries(turned)) {
    assert.equal(choose(line, name), `Intro\n\n${markdown}\n\nOutro`, name)
  }
  const item = '- One\n- /xTwo\n- Three'
  assert.equal(choose(item, 'Text'), '- One\n\nTwo\n\n- Three')
  assert.equal(choose(item, 'Heading 2'), '- One\n\n## Two\n\n- Three')
  assert.equal(choose(item, 'Numbered list'), '- One\n\n1. Two\n\n- Three')
  assert.equal(choose(item, 'Checklist'), '- One\n\n- [ ] Two\n\n- Three')
  assert.equal(choose(item, 'Quote'), '- One\n\n> Two\n\n- Three')
  assert.equal(choose('1. One\n2. /xTwo\n3. Three', 'Bulleted list'), '1. One\n\n- Two\n\n1. Three')
  assert.equal(choose('> /xBring hard hats', 'Checklist'), '- [ ] Bring hard hats')
})

test('the / menu puts a divider or table in place of an empty line', () => {
  assert.equal(choose('Intro\n\n/x\n\nOutro', 'Divider'), 'Intro\n\n---\n\nOutro')
  const table = open(choose('Intro\n\n/x\n\nOutro', 'Table'))
  const { content } = table.state.doc.content
  assert.deepEqual(content.map((node) => node.type.name), ['paragraph', 'table', 'paragraph'])
  assert.deepEqual(content[1].content.content.map((row) => row.content.content.map((cell) => cell.type.name)), [
    ['tableHeader', 'tableHeader', 'tableHeader'],
    ['tableCell', 'tableCell', 'tableCell'],
    ['tableCell', 'tableCell', 'tableCell'],
  ])
  table.destroy()
})

test('block handles move, copy, delete and convert one list item at a time', () => {
  const markdown = '- [ ] Book flights\n  - [ ] Verify passport\n- [ ] Compare hotels\n\nNotes'
  const editor = open(markdown)
  const passport = blockAt(editor.state.doc, textblock(editor, 'Verify passport'))
  assert.equal(editor.state.doc.nodeAt(passport)!.textContent, 'Verify passport')
  assert.equal(kindOf(editor.state.doc, passport)?.name, 'Checklist')
  assert.equal(blockAt(editor.state.doc, textblock(editor, 'Notes')), textblock(editor, 'Notes'))

  duplicateBlock(editor, passport)
  assert.equal(saved(editor), '- [ ] Book flights\n  - [ ] Verify passport\n  - [ ] Verify passport\n- [ ] Compare hotels\n\nNotes')
  assert.equal(deleteBlock(editor, passport), true)
  assert.equal(deleteBlock(editor, blockAt(editor.state.doc, textblock(editor, 'Verify passport'))), true)
  assert.equal(saved(editor), '- [ ] Book flights\n- [ ] Compare hotels\n\nNotes')

  assert.equal(addBelow(editor, blockAt(editor.state.doc, textblock(editor, 'Compare hotels'))), true)
  assert.equal(editor.state.selection.$from.parent.textContent, '/')
  assert.equal(editor.state.selection.$from.parentOffset, 1)
  assert.equal(saved(editor), '- [ ] Book flights\n- [ ] Compare hotels\n- [ ] /\n\nNotes')

  assert.equal(turnBlock(editor, blockAt(editor.state.doc, textblock(editor, 'Compare hotels')), blocks.find((block) => block.name === 'Heading 1')!), true)
  assert.equal(saved(editor), '- [ ] Book flights\n\n# Compare hotels\n\n- [ ] /\n\nNotes')

  assert.equal(addBelow(editor, textblock(editor, 'Notes')), true)
  assert.equal(saved(editor), '- [ ] Book flights\n\n# Compare hotels\n\n- [ ] /\n\nNotes\n\n/')
  editor.destroy()
})

test('callouts keep their icon, color and body through saving, beside plain quotes', () => {
  const markdown = 'Intro\n\n> [!callout|blue] 🚚\n> Ship by **Friday**\n>\n> - Pallets\n> - Labels\n\n> [!callout]\n> No icon\n\n> Plain quote'
  const editor = open(markdown)
  const [, shipping, bare, quote] = editor.state.doc.content.content
  assert.deepEqual([shipping.type.name, shipping.attrs.icon, shipping.attrs.color], ['callout', '🚚', 'blue'])
  assert.deepEqual(shipping.content.content.map((node) => node.type.name), ['paragraph', 'bulletList'])
  assert.deepEqual([bare.type.name, bare.attrs.icon, bare.attrs.color], ['callout', '', 'default'])
  assert.equal(quote.type.name, 'blockquote')
  assert.equal(saved(editor), markdown)
  editor.destroy()
})
