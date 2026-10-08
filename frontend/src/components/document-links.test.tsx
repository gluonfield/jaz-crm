import assert from 'node:assert/strict'
import { test } from 'node:test'
import { Editor } from '@tiptap/core'
import { DocumentKit, documentLinkAt, editDocumentLink } from './document-links'

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

test('splitting a URL bullet preserves its text and saves two separate bullets', () => {
  const url = 'https://demo.cambridgeadvancedsystems.com/compact-briggs-die'
  const editor = new Editor({ element: null, extensions: [DocumentKit], content: '- Start', contentType: 'markdown' })
  editor.view.updateState(editor.state.reconfigure({ plugins: editor.extensionManager.plugins }))
  editor.view.dispatch(editor.state.tr.insertText(url, 3, 8))
  editor.commands.setTextSelection(3 + url.length)
  assert.equal(editor.commands.splitListItem('listItem'), true)
  editor.view.dispatch(editor.state.tr.insertText('Next bullet'))
  assert.equal(editor.state.doc.firstChild!.childCount, 2)
  assert.equal(editor.state.doc.firstChild!.child(0).textContent, url)
  assert.equal(editor.state.doc.firstChild!.child(1).textContent, 'Next bullet')
  const restored = new Editor({ element: null, extensions: [DocumentKit], content: editor.getMarkdown(), contentType: 'markdown' })
  assert.equal(restored.state.doc.firstChild!.child(0).textContent, url)
  assert.equal(restored.state.doc.firstChild!.child(1).textContent, 'Next bullet')
  restored.destroy()
  editor.destroy()
})

test('editing one link preserves formatting and repeated links, with atomic undo and safe destinations', () => {
  const markdown = '[**Visitor** guide](https://example.com/guide) and [Visitor guide](https://example.com/guide)'
  const editor = new Editor({ element: null, extensions: [DocumentKit], content: markdown, contentType: 'markdown' })
  editor.view.updateState(editor.state.reconfigure({ plugins: editor.extensionManager.plugins }))
  const original = editor.getJSON()
  const first = documentLinkAt(editor, 2)!
  assert.equal(editDocumentLink(editor, first, first.text, 'https://example.com/new'), true)
  assert.match(editor.getMarkdown(), /\[\*\*Visitor\*\* guide\]\(https:\/\/example.com\/new\)/)
  assert.match(editor.getMarkdown(), /\[Visitor guide\]\(https:\/\/example.com\/guide\)/)
  assert.equal(editor.commands.undo(), true)
  assert.deepEqual(editor.getJSON(), original)
  const renamed = documentLinkAt(editor, 2)!
  assert.equal(editDocumentLink(editor, renamed, 'Demo', '/r/target-page'), true)
  assert.match(editor.getMarkdown(), /\[\*\*Demo\*\*\]\(\/r\/target-page\)/)
  assert.match(editor.getMarkdown(), /\[Visitor guide\]\(https:\/\/example.com\/guide\)/)
  const current = documentLinkAt(editor, 2)!
  const beforeRejected = editor.getJSON()
  assert.equal(editDocumentLink(editor, current, 'Danger', 'javascript:alert(1)'), false)
  assert.deepEqual(editor.getJSON(), beforeRejected)
  editor.view.dispatch(editor.state.tr.insertText('Prefix ', 1))
  const afterTyping = editor.getJSON()
  assert.equal(editDocumentLink(editor, current, 'Old edit', 'https://example.com/stale'), false)
  assert.deepEqual(editor.getJSON(), afterTyping)
  const restored = new Editor({ element: null, extensions: [DocumentKit], content: editor.getMarkdown(), contentType: 'markdown' })
  assert.deepEqual(restored.getJSON(), editor.getJSON())
  restored.destroy()
  editor.destroy()
})
