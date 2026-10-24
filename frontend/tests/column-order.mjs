import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { execFileSync } from 'node:child_process'
import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { browser, click, until } from './browser.mjs'

const directory = resolve('../runs/column-order-tests')

test('table order follows drags and keyboard menus, survives reload, and rolls back failed saves', { timeout: 60000 }, async () => {
  await mkdir(directory, { recursive: true })
  execFileSync('bun', ['build', 'tests/column-order.tsx', '--outdir', directory, '--target', 'browser'], { stdio: 'pipe' })
  const bundle = await readFile(resolve(directory, 'column-order.js'))
  const assets = resolve('dist/client/assets')
  const sheet = (await readdir(assets)).find((file) => /^styles-.*\.css$/.test(file))
  assert.ok(sheet, 'run bun run build first for the app stylesheet')
  const styles = await readFile(resolve(assets, sheet))
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
  let releaseFailure
  const server = createServer(async (req, res) => {
    if (req.url === '/bundle.js' || req.url === '/styles.css') {
      res.setHeader('Content-Type', req.url === '/bundle.js' ? 'text/javascript' : 'text/css')
      res.end(req.url === '/bundle.js' ? bundle : styles)
      return
    }
    if (!req.url.startsWith('/api/tools/')) {
      res.setHeader('Content-Type', 'text/html')
      res.end('<meta charset="utf-8"><link rel="stylesheet" href="/styles.css"><div id="root"></div><script src="/bundle.js"></script>')
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
        await new Promise((accept) => { releaseFailure = accept })
        res.writeHead(500)
        res.end(JSON.stringify({ error: 'Synthetic save failure' }))
      } else {
        objects.find((object) => object.slug === args.object).columns = args.columns
        res.end('{}')
      }
      return
    }
    const response = tool === 'list_objects' ? { objects } : tool === 'search_records' ? { records: Array.from({ length: 12 }, (_, index) => ({ id: `example-${index}`, object: args.object, created_at: '2026-10-19T12:00:00Z', updated_at: '2026-10-19T13:00:00Z', values: { name: `Example ${index + 1}`, location: 'Cambridge', cost: '45', updated_at: 'Custom value' } })) } : {}
    res.end(JSON.stringify(response))
  })
  await new Promise((accept) => server.listen(0, '127.0.0.1', accept))
  const chrome = await browser(directory)
  try {
    const url = `http://127.0.0.1:${server.address().port}`
    const page = await chrome.page(url)
    const headers = () => page.evaluate(`Array.from(document.querySelectorAll('#root th[data-column]'), n => n.dataset.column)`)
    const ready = () => until(async () => (await headers()).length > 0)
    await ready()
    assert.deepEqual(await headers(), ['name', '$updated_at', 'location', 'cost', 'updated_at'])
    assert.equal(await page.evaluate(`document.querySelector('th[data-column="name"]').draggable`), false)
    const initial = await headers()
    const point = (key, fraction = 0.5) => page.evaluate(`(() => {
      const rect = document.querySelector('th[data-column="${key}"]').getBoundingClientRect()
      return { x: rect.x + rect.width * ${fraction}, y: rect.y + rect.height / 2 }
    })()`)
    const press = async (at) => {
      await page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', ...at })
      await page.call('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', buttons: 1, clickCount: 1, ...at })
    }
    const move = (at) => page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', button: 'left', buttons: 1, ...at })
    const release = (at) => page.call('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'left', clickCount: 1, ...at })
    const source = await point('cost')
    const target = await point('location', 0.2)
    await press(source)
    await move({ ...source, x: source.x + 3 })
    assert.equal(await page.evaluate(`!!document.querySelector('[data-column-dragging]')`), false, 'clicks have a drag threshold')
    await release(source)
    assert.equal(writes.length, 0)
    await press(source)
    await move(target)
    await until(() => page.evaluate(`!!document.querySelector('td[data-column-moving]')`))
    await page.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape' })
    await page.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape' })
    await release(target)
    assert.equal(await page.evaluate(`!!document.querySelector('[data-column-dragging], [data-column-moving]')`), false)
    assert.deepEqual(await headers(), initial, 'Escape restores the original order')
    assert.equal(writes.length, 0, 'cancellation does not save')
    await press(source)
    await move(target)
    await until(() => page.evaluate(`Math.abs(document.querySelector('th[data-column="location"]').getBoundingClientRect().left - 600) < 1`))
    assert.deepEqual(await headers(), initial, 'the saved order stays unchanged until release')
    assert.equal(writes.length, 0)
    const moving = await page.evaluate(`Array.from(document.querySelectorAll('.table-column-preview [data-column]'), cell => ({ key: cell.dataset.column, x: cell.getBoundingClientRect().left }))`)
    assert.equal(moving.length, 13, 'the entire column follows the pointer')
    assert.ok(moving.every((cell) => cell.key === 'cost' && Math.abs(cell.x - moving[0].x) < 1), 'header and row cells stay aligned')
    await writeFile(resolve(directory, 'drag.png'), Buffer.from((await page.call('Page.captureScreenshot')).data, 'base64'))
    await release(target)
    await until(async () => (await headers())[2] === 'cost' && writes.length === 1)
    assert.deepEqual(writes[0].columns, ['archived', '$updated_at', 'cost', 'location', 'updated_at'])
    assert.deepEqual(await page.evaluate(`Array.from(document.querySelectorAll('tbody tr:first-child td[data-column]'), n => n.dataset.column)`), await headers())
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
    await until(() => page.evaluate(`getComputedStyle(document.querySelector('th[data-column="cost"]')).cursor === 'grab'`))
    await until(() => page.evaluate(`document.activeElement?.getAttribute('aria-label') === 'Cost column options'`), 'Column menu must restore focus after closing')
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
    await until(async () => writes.length === 3 && JSON.stringify(await headers()) !== JSON.stringify(prior))
    assert.notDeepEqual(await headers(), prior, 'a pending save must show its optimistic order')
    releaseFailure()
    await until(async () => writes.length === 3 && JSON.stringify(await headers()) === JSON.stringify(prior), 'Failed column save must restore the server order')
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
      await until(() => page.evaluate(`getComputedStyle(document.querySelector('th[data-column="cost"]')).cursor === 'grab'`))
      const next = await headers()
      const from = await point('location')
      const to = await point('cost', 0.1)
      await press(from)
      await move(to)
      await until(() => page.evaluate(`!!document.querySelector('td[data-column-moving]')`))
      await release(to)
      await until(async () => writes.length === start + 2 && JSON.stringify(await headers()) !== JSON.stringify(next))
      assert.notDeepEqual(await headers(), next, `${object} uses the same direct column drag`)
      assert.equal((await headers())[0], 'name')
    }
    await page.call('Page.navigate', { url: `${url}/?object=companies` })
    await ready()
    const companyOrder = await headers()
    const wide = await point('$people')
    const last = await point(companyOrder.at(-1), 0.95)
    await press(wide)
    await move(last)
    await until(() => page.evaluate(`!!document.querySelector('.table-column-preview')`))
    await release(last)
    await until(async () => (await headers()).at(-1) === '$people')
    await until(() => page.evaluate(`getComputedStyle(document.querySelector('th[data-column="$people"]')).cursor === 'grab'`))
    await page.call('Emulation.setDeviceMetricsOverride', { width: 640, height: 600, deviceScaleFactor: 1, mobile: false })
    const beforeScroll = await headers()
    const atEdge = await point(beforeScroll[1])
    await press(atEdge)
    await move({ x: 630, y: atEdge.y })
    await until(() => page.evaluate(`document.querySelector('table').parentElement.scrollLeft > 100`))
    await page.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] })
    assert.equal(await page.evaluate(`getComputedStyle(document.querySelector('th[data-column="location"]')).transitionDuration`), '0s')
    await key('Escape', 'Escape')
    await release({ x: 630, y: atEdge.y })
    assert.deepEqual(await headers(), beforeScroll, 'cancelling an edge-scrolling drag preserves order')
    for (const theme of ['light', 'dark']) {
      await page.evaluate(`document.documentElement.classList.toggle('dark', ${theme === 'dark'})`)
      await page.evaluate(`document.querySelector('table').parentElement.scrollLeft = 340`)
      const frozen = await page.evaluate(`(() => {
        const cell = document.querySelector('tbody td[data-column="name"]')
        cell.parentElement.dataset.focused = 'true'
        const style = getComputedStyle(cell)
        return { x: cell.getBoundingClientRect().left, color: style.backgroundColor, overlay: style.backgroundImage }
      })()`)
      assert.equal(frozen.x, 0, 'name stays pinned during horizontal scroll')
      assert.ok(!frozen.color.includes('rgba') && frozen.color !== 'transparent', 'focused frozen cells have a solid base behind the highlight')
      assert.notEqual(frozen.overlay, 'none', 'the frozen highlight matches the row')
      await page.evaluate(`delete document.querySelector('tbody tr').dataset.focused`)
      await page.evaluate(`document.querySelector('tbody tr').dataset.state = 'open'`)
      assert.equal(await page.evaluate(`getComputedStyle(document.querySelector('tbody td[data-column="name"]')).backgroundColor`), frozen.color, 'context menus keep the frozen cell opaque')
      await page.evaluate(`delete document.querySelector('tbody tr').dataset.state`)
      await page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 50, y: 55 })
      assert.equal(await page.evaluate(`getComputedStyle(document.querySelector('tbody td[data-column="name"]')).backgroundColor`), frozen.color, 'hover keeps the frozen cell opaque')
      await writeFile(resolve(directory, `scroll-${theme}.png`), Buffer.from((await page.call('Page.captureScreenshot')).data, 'base64'))
    }
  } finally {
    releaseFailure?.()
    await chrome.close()
    await new Promise((accept) => server.close(accept))
  }
})
