import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { execFileSync } from 'node:child_process'
import { mkdir, readFile, readdir } from 'node:fs/promises'
import { resolve } from 'node:path'
import { browser, until } from './browser.mjs'

const directory = resolve('../runs/data-drag-tests')

test('a Data row drags as itself, solid under the pointer, fades while it moves, and ends at once when let go in place', { timeout: 60000 }, async () => {
  await mkdir(directory, { recursive: true })
  execFileSync('bun', ['build', 'tests/data-drag.tsx', '--outdir', directory, '--target', 'browser'], { stdio: 'pipe' })
  const bundle = await readFile(resolve(directory, 'data-drag.js'))
  // The built stylesheet carries the row's real fade and transition.
  const assets = resolve('dist/client/assets')
  const sheet = (await readdir(assets)).find((file) => /^styles-.*\.css$/.test(file))
  assert.ok(sheet, 'run bun run build first for the app stylesheet')
  const styles = await readFile(resolve(assets, sheet))
  const made = '2026-10-19T12:00:00Z'
  const records = [{ id: 'company', object: 'pages', created_at: made, updated_at: made, values: { name: 'Company' } }]
  const objects = [{ slug: 'events', name: 'Events', position: Date.parse(made) - 1000, attributes: [] }]
  const writes = []
  const server = createServer(async (req, res) => {
    if (req.url === '/bundle.js' || req.url === '/styles.css') {
      res.setHeader('Content-Type', req.url === '/bundle.js' ? 'text/javascript' : 'text/css')
      res.end(req.url === '/bundle.js' ? bundle : styles)
      return
    }
    if (!req.url.startsWith('/api/tools/')) {
      res.setHeader('Content-Type', 'text/html')
      res.end('<meta charset="utf-8"><link rel="stylesheet" href="/styles.css"><div id="root" style="width:240px"></div><script src="/bundle.js"></script>')
      return
    }
    let raw = ''
    for await (const chunk of req) {
      raw += chunk
    }
    const tool = req.url.split('/').at(-1)
    if (tool === 'edit_object') {
      writes.push(JSON.parse(raw))
    }
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify(tool === 'list_objects' ? { objects } : tool === 'search_records' ? { records } : {}))
  })
  await new Promise((accept) => server.listen(0, '127.0.0.1', accept))
  const chrome = await browser(directory)
  try {
    const page = await chrome.page(`http://127.0.0.1:${server.address().port}`)
    await until(() => page.evaluate(`!!document.querySelector('a[href="/r/company"]')`))
    await page.evaluate(`document.addEventListener('dragstart', (event) => {
      const row = event.target.closest('[draggable="true"]')
      window.dragged = { source: event.target.tagName }
      setTimeout(() => { window.dragged.opacity = getComputedStyle(row).opacity })
    }, true)
    window.addEventListener('dragover', (event) => { window.taken = event.defaultPrevented })`)
    const point = (href, fraction) => page.evaluate(`(() => {
      const rect = document.querySelector('a[href="${href}"]').getBoundingClientRect()
      return { x: rect.x + 40, y: rect.y + rect.height * ${fraction} }
    })()`)
    const source = await point('/o/events', 0.5)
    const target = await point('/r/company', 0.85)
    let drag
    chrome.on('Input.dragIntercepted', (event) => {
      drag = event.data
    })
    const lift = async (from, to) => {
      drag = undefined
      await page.call('Input.setInterceptDrags', { enabled: true })
      await page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', ...from })
      await page.call('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', buttons: 1, clickCount: 1, ...from })
      await page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', button: 'left', buttons: 1, x: from.x, y: from.y + 6 })
      await page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', button: 'left', buttons: 1, ...to })
      await until(() => drag)
      await page.call('Input.dispatchDragEvent', { type: 'dragEnter', ...to, data: drag })
      await page.call('Input.dispatchDragEvent', { type: 'dragOver', ...to, data: drag })
    }
    const release = async (at) => {
      await page.call('Input.dispatchDragEvent', { type: 'drop', ...at, data: drag })
      await page.call('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'left', clickCount: 1, ...at })
    }
    await lift(source, target)
    await until(() => page.evaluate(`getComputedStyle(document.querySelector('a[href="/o/events"]').parentElement).opacity === '0.5'`))
    assert.deepEqual(await page.evaluate('window.dragged'), { source: 'DIV', opacity: '1' }, 'the row, not its link, is dragged and pictured before it fades')
    await release(target)
    await until(() => writes.length === 1)
    assert.deepEqual(writes[0], { object: 'events', action: 'move', position: Date.parse(made) + 1000 }, 'a move beside a page at the top level keeps the parent it has')
    await until(() => page.evaluate(`document.querySelector('a[href="/o/events"]').parentElement.draggable`))
    // Let go on its own row: the page takes the drop, so the browser ends the
    // drag at once instead of sliding the picture back, and nothing moves.
    const still = await point('/o/events', 0.5)
    await lift(still, { x: still.x, y: still.y + 40 })
    await page.call('Input.dispatchDragEvent', { type: 'dragOver', ...still, data: drag })
    assert.equal(await page.evaluate('window.taken'), true, 'a drag let go in place is taken')
    await release(still)
    await until(() => page.evaluate(`getComputedStyle(document.querySelector('a[href="/o/events"]').parentElement).opacity === '1'`))
    assert.equal(writes.length, 1)
  } finally {
    await chrome.close()
    await new Promise((accept) => server.close(accept))
  }
})
