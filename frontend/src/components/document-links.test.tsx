import assert from 'node:assert/strict'
import { test } from 'node:test'
import { Editor } from '@tiptap/core'
import { DocumentKit } from './document-links'

test('page and read-only documents preserve links, dividers and nested checklists through edits', () => {
  const markdown = '[Budget](/r/budget) and [Visitor guide](https://example.com/guide)\n\n---\n\n- [x] **Book flights**\n  - [ ] Verify passport\n  - Packing list\n- [ ] Compare hotels'
  for (const editable of [true, false]) {
    const options = { element: null, editable, extensions: [DocumentKit.configure(editable ? { link: { openOnClick: false } } : { trailingNode: false })], contentType: 'markdown' as const }
    const editor = new Editor({ ...options, content: markdown })
    let checked = 0
    let unchecked = 0
    let divider = 0
    let firstTask: number | undefined
    const links: string[] = []
    editor.state.doc.check()
    editor.state.doc.descendants((node, position) => {
      if (node.type.name === 'taskItem') {
        firstTask ??= position
        if (node.attrs.checked) {
          checked++
        } else {
          unchecked++
        }
      }
      if (node.type.name === 'horizontalRule') {
        divider++
      }
      for (const mark of node.marks) {
        if (mark.type.name === 'link') {
          links.push(mark.attrs.href)
        }
      }
    })
    assert.equal(checked, 1)
    assert.equal(unchecked, 2)
    assert.equal(divider, 1)
    assert.deepEqual(links, ['/r/budget', 'https://example.com/guide'])
    assert.match(editor.state.doc.textContent, /Packing list/)
    if (editable) {
      assert.notEqual(firstTask, undefined)
      editor.view.dispatch(editor.state.tr.setNodeMarkup(firstTask!, undefined, { checked: false }))
      assert.match(editor.getMarkdown(), /- \[ \] \*\*Book flights\*\*/)
    }
    const roundTrip = new Editor({ ...options, content: editor.getMarkdown() })
    assert.deepEqual(roundTrip.getJSON(), editor.getJSON())
    assert.equal(roundTrip.getMarkdown(), editor.getMarkdown())
    roundTrip.destroy()
    editor.destroy()
  }
})
