import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { execFileSync } from 'node:child_process'
import { mkdir, readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { browser, click, until } from './browser.mjs'

const directory = resolve('../runs/column-order-tests')

test('table order follows drags and keyboard menus, survives reload, and rolls back failed saves', { timeout: 60000 }, async () => {
  await mkdir(directory, { recursive: true })
  execFileSync('bun', ['build', 'tests/column-order.tsx', '--outdir', directory, '--target', 'browser'], { stdio: 'pipe' })
  const bundle = await readFile(resolve(directory, 'column-order.js'))
  const objects = ['events', 'people', 'companies', 'deals', 'follow_ups'].map((slug) => ({
    slug, name: slug, columns: slug === 'events' ? ['archived', '$updated_at'] : [], standard: slug !== 'events', attributes: [
      { slug: 'name', name: 'Name', type: 'text', protected: true },
      { slug: 'location', name: 'Location', type: 'text' },
      { slug: 'cost', name: 'Cost', type: 'number' },
      { slug: 'updated_at', name: 'Custom timestamp', type: 'text' },
    ],
  }))
  const writes = []
  let fail = false
  const server = createServer(async (req, res) => {
    if (req.url === '/bundle.js') {
      res.setHeader('Content-Type', 'text/javascript')
      res.end(bundle)
      return
    }
    if (!req.url.startsWith('/api/tools/')) {
      res.setHeader('Content-Type', 'text/html')
      res.end('<meta charset="utf-8"><style>body{margin:0}table{width:100%;table-layout:fixed;border-collapse:collapse}th,td{position:relative;height:36px;text-align:left}th>div{display:flex}button{min-width:24px;min-height:24px}[data-column-drop]{position:absolute;top:0;bottom:0;width:2px;background:red}</style><div id="root"></div><script src="/bundle.js"></script>')
      return
    }
    let raw = ''
    for await (const chunk of req) {
      raw += chunk
    }
    const args = JSON.parse(raw || '{}')
    const tool = req.url.split('/').at(-1)
    res.setHeader('Content-Type', 'application/json')
    if (tool === 'edit_object') {
      writes.push(args)
      if (fail) {
        res.writeHead(500)
        res.end(JSON.stringify({ error: 'Synthetic save failure' }))
      } else {
        objects.find((object) => object.slug === args.object).columns = args.columns
        res.end('{}')
      }
      return
    }
    const response = tool === 'list_objects' ? { objects } : tool === 'search_records' ? { records: [{ id: 'example', object: args.object, created_at: '2026-10-19T12:00:00Z', updated_at: '2026-10-19T13:00:00Z', values: { name: 'Example', location: 'Cambridge', cost: '45', updated_at: 'Custom value' } }] } : {}
    res.end(JSON.stringify(response))
  })
  await new Promise((accept) => server.listen(0, '127.0.0.1', accept))
  const chrome = await browser(directory)
  try {
    const url = `http://127.0.0.1:${server.address().port}`
    const page = await chrome.page(url)
    const headers = () => page.evaluate(`Array.from(document.querySelectorAll('th[data-column]'), n => n.dataset.column)`)
    const ready = () => until(async () => (await headers()).length > 0)
    await ready()
    assert.deepEqual(await headers(), ['name', '$updated_at', 'location', 'cost', 'updated_at'])
    assert.equal(await page.evaluate(`document.querySelector('th[data-column="name"]').draggable`), false)
    const initial = await headers()
    const point = (key) => page.evaluate(`(() => {
      const rect = document.querySelector('th[data-column="${key}"]').getBoundingClientRect()
      return { x: rect.x + 10, y: rect.y + rect.height / 2 }
    })()`)
    const source = await point('cost')
    const target = await point('location')
    let drag
    chrome.on('Input.dragIntercepted', (event) => {
      drag = event.data
    })
    await page.call('Input.setInterceptDrags', { enabled: true })
    await page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', ...source })
    await page.call('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', buttons: 1, clickCount: 1, ...source })
    await page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', button: 'left', buttons: 1, x: source.x - 20, y: source.y })
    await page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', button: 'left', buttons: 1, ...target })
    await until(() => drag)
    await page.call('Input.dispatchDragEvent', { type: 'dragEnter', ...target, data: drag })
    await page.call('Input.dispatchDragEvent', { type: 'dragOver', ...target, data: drag })
    await until(() => page.evaluate(`!!document.querySelector('[data-column-drop]')`))
    assert.deepEqual(await headers(), initial, 'hover must not move the columns')
    await page.call('Input.dispatchDragEvent', { type: 'drop', ...target, data: drag })
    await page.call('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'left', clickCount: 1, ...target })
    await until(async () => (await headers())[2] === 'cost' && writes.length === 1)
    assert.deepEqual(writes[0].columns, ['archived', '$updated_at', 'cost', 'location', 'updated_at'])
    assert.deepEqual(await page.evaluate(`Array.from(document.querySelectorAll('tbody td[data-column]'), n => n.dataset.column)`), await headers())
    await page.call('Page.reload')
    await ready()
    assert.deepEqual(await headers(), ['name', '$updated_at', 'cost', 'location', 'updated_at'])
    // Focus the actual header menu, then use only keyboard input to move it.
    await page.evaluate(`document.querySelector('[aria-label="Cost column options"]').focus()`)
    const key = async (key, code) => {
      await page.call('Input.dispatchKeyEvent', { type: 'keyDown', key, code })
      await page.call('Input.dispatchKeyEvent', { type: 'keyUp', key, code })
    }
    await key('Enter', 'Enter')
    await until(() => page.evaluate(`!!document.querySelector('[role="menu"]')`))
    await key('Home', 'Home')
    await key('Enter', 'Enter')
    await until(async () => writes.length === 2 && (await headers())[1] === 'cost')
    assert.deepEqual(writes[1].columns, ['archived', 'cost', '$updated_at', 'location', 'updated_at'])
    assert.equal(await page.evaluate(`document.activeElement?.getAttribute('aria-label')`), 'Cost column options')
    await key('Enter', 'Enter')
    await until(() => page.evaluate(`!!document.querySelector('[role="menu"]')`))
    assert.equal(await page.evaluate(`Array.from(document.querySelectorAll('[role="menuitem"]')).find(n => n.textContent.includes('Move Left')).getAttribute('aria-disabled')`), 'true')
    await key('Escape', 'Escape')
    await until(() => page.evaluate(`!document.querySelector('[role="menu"]')`))
    const prior = await headers()
    fail = true
    await click(page, '[aria-label="Location column options"]')
    await until(() => page.evaluate(`!!document.querySelector('[role="menu"]')`))
    await page.evaluate(`Array.from(document.querySelectorAll('[role="menuitem"]')).find(n=>n.textContent.includes('Move Left')).click()`)
    await until(async () => writes.length === 3 && JSON.stringify(await headers()) === JSON.stringify(prior))
    assert.deepEqual(await headers(), prior, 'a failed write must restore the server order')
    fail = false
    for (const object of ['people', 'companies', 'deals', 'follow_ups']) {
      await page.call('Page.navigate', { url: `${url}/?object=${object}` })
      await ready()
      const before = await headers()
      const start = writes.length
      await click(page, '[aria-label="Cost column options"]')
      await until(() => page.evaluate(`!!document.querySelector('[role="menu"]')`))
      await page.evaluate(`Array.from(document.querySelectorAll('[role="menuitem"]')).find(n=>n.textContent.includes('Move Right')).click()`)
      await until(async () => writes.length > start && JSON.stringify(await headers()) !== JSON.stringify(before))
      assert.equal(writes.at(-1).object, object)
      assert.equal((await headers())[0], 'name')
    }
  } finally {
    await chrome.close()
    await new Promise((accept) => server.close(accept))
  }
})
