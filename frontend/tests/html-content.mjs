import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { execFileSync } from 'node:child_process'
import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { browser, click, until } from './browser.mjs'

const directory = resolve('../runs/security/fixes/email-browser')

test('email images require explicit consent for each message', { timeout: 60000 }, async () => {
  await mkdir(directory, { recursive: true })
  execFileSync('bun', ['build', 'tests/html-content.tsx', '--outdir', directory, '--target', 'browser'], { stdio: 'pipe' })
  const bundle = await readFile(resolve(directory, 'html-content.js'))
  const assets = resolve('dist/client/assets')
  const sheet = (await readdir(assets)).find((name) => /^styles-.*\.css$/.test(name))
  const css = await readFile(resolve(assets, sheet))
  const server = createServer((req, res) => {
    if (req.url === '/bundle.js' || req.url === '/styles.css') {
      res.setHeader('Content-Type', req.url === '/bundle.js' ? 'text/javascript' : 'text/css')
      res.end(req.url === '/bundle.js' ? bundle : css)
      return
    }
    res.setHeader('Content-Type', 'text/html')
    res.end('<meta charset="utf-8"><link rel="stylesheet" href="/styles.css"><div id="root"></div><script src="/bundle.js"></script>')
  })
  await new Promise((accept) => server.listen(0, '127.0.0.1', accept))
  const chrome = await browser(directory)
  const requests = []
  chrome.on('Fetch.requestPaused', (event, sessionId) => {
    requests.push(event.request.url)
    void chrome.send('Fetch.failRequest', { requestId: event.requestId, errorReason: 'BlockedByClient' }, sessionId)
  })
  try {
    const page = await chrome.page('about:blank')
    await page.call('Fetch.enable', { patterns: [{ urlPattern: 'https://tracker.invalid/*' }] })
    await page.call('Page.navigate', { url: `http://127.0.0.1:${server.address().port}` })
    await until(() => page.evaluate(`document.body.textContent.includes('Remote images hidden')`))
    await new Promise((accept) => setTimeout(accept, 300))
    assert.equal(requests.length, 0, 'parsing and rendering must not request tracking images')
    assert.equal(await page.evaluate(`document.querySelectorAll('#root img, #root iframe, #root script').length`), 0)
    assert.equal(await page.evaluate(`window.unsafe === undefined`), true)
    const load = '#root button:last-child'
    await click(page, load)
    await until(() => requests.length === 1)
    assert.equal(requests[0], 'https://tracker.invalid/pixel?message=first')
    assert.equal(await page.evaluate(`document.querySelector('#root img').hasAttribute('onerror')`), false)
    await click(page, '#next')
    await until(() => page.evaluate(`document.body.textContent.includes('Second message') && document.body.textContent.includes('Remote images hidden')`))
    await new Promise((accept) => setTimeout(accept, 300))
    assert.equal(requests.length, 1, 'consent must not carry over to the next message')
    assert.equal(await page.evaluate(`document.querySelectorAll('#root img').length`), 0)
    for (const [name, width, dark] of [['light', 1280, false], ['dark-narrow', 390, true]]) {
      await page.call('Emulation.setDeviceMetricsOverride', { width, height: 700, deviceScaleFactor: 1, mobile: width < 500 })
      await page.evaluate(`document.documentElement.classList.toggle('dark', ${dark})`)
      const shot = await page.call('Page.captureScreenshot', { format: 'png' })
      await writeFile(resolve(directory, name + '.png'), Buffer.from(shot.data, 'base64'))
      assert.equal(await page.evaluate(`document.documentElement.scrollWidth <= innerWidth`), true)
    }
    await click(page, load)
    await until(() => requests.length === 2)
    assert.equal(requests[1], 'https://tracker.invalid/pixel?message=second')
    assert.equal(await page.evaluate(`window.unsafe === undefined`), true)
  } finally {
    await chrome.close()
    await new Promise((accept) => server.close(accept))
  }
})
