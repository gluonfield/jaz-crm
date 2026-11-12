import test from 'node:test'
import assert from 'node:assert/strict'
import { spawn, execFileSync } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { createInterface } from 'node:readline'
import { browser, until } from './browser.mjs'

const directory = resolve('../runs/operational-status/browser')

test('General reports persisted quota pauses and recovery through live changes', { timeout: 180000 }, async () => {
  await mkdir(directory, { recursive: true })
  const binary = resolve(directory, 'server')
  execFileSync('go', ['build', '-o', binary, './internal/httpapi/authapi/testdata/browser'], { cwd: resolve('..'), stdio: 'pipe' })
  const server = spawn(binary, [], { env: { ...process.env, SHORTCUT_WEB_DIR: process.env.SHORTCUT_WEB_DIR ?? resolve('dist/client') }, stdio: ['pipe', 'pipe', 'inherit'] })
  const exited = new Promise((accept) => server.once('close', accept))
  const lines = createInterface({ input: server.stdout })
  let chrome
  try {
    const config = await Promise.race([
      new Promise((accept, reject) => {
        lines.once('line', (line) => accept(JSON.parse(line)))
        server.once('exit', (code) => reject(new Error(`Fixture exited ${code}`)))
      }),
      new Promise((_, reject) => setTimeout(() => reject(new Error('Fixture startup timed out')), 20000).unref()),
    ])
    chrome = await browser(directory)
    let streamReady = false
    let changes = 0
    chrome.on('Network.responseReceived', ({ response }) => {
      if (response.url === `${config.url}/api/changes` && response.status === 200) {
        streamReady = true
      }
    })
    chrome.on('Network.eventSourceMessageReceived', ({ data }) => {
      if (data === 'changed') {
        changes++
      }
    })
    const page = await chrome.page('about:blank')
    await page.call('Network.setCookie', { name: 'jc_session', value: config.session, url: config.url })
    await page.call('Page.navigate', { url: `${config.url}/settings` })
    await until(() => page.evaluate('!!document.querySelector("#workspace-name") && !!document.querySelector("#workspace-timezone") && !document.body.textContent.includes("Checking for reported issues")'))
    await until(() => streamReady, 'General must subscribe to live changes before the status changes')
    const general = () => page.evaluate('({ name: document.querySelector("#workspace-name").value, timezone: document.querySelector("#workspace-timezone").value, path: location.pathname, loadedAt: performance.timeOrigin })')
    const initial = await general()
    const attention = () => page.evaluate('Array.from(document.querySelectorAll("section")).find(section => section.querySelector("h2")?.textContent === "Needs attention")?.innerText')
    assert.equal(await attention(), undefined)
    server.stdin.write(JSON.stringify({ status: 'quota' }) + '\n')
    await until(async () => /AI background work/.test(await attention()), 'A persisted quota pause must appear without reloading General')
    assert.match(await attention(), /credit or usage limit/)
    assert.match(await attention(), /Last occurred.*Next check/s)
    assert.ok(changes > 0, 'The quota pause must arrive through the real change stream')
    for (const scheme of ['light', 'dark']) {
      await page.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: scheme }] })
      await until(() => page.evaluate(`document.documentElement.classList.contains('dark') === ${scheme === 'dark'}`))
      for (const width of [1440, 390]) {
        await page.call('Emulation.setDeviceMetricsOverride', { width, height: 900, deviceScaleFactor: 1, mobile: false })
        assert.equal(await page.evaluate('document.documentElement.scrollWidth <= innerWidth'), true, 'General must fit a narrow viewport')
        assert.deepEqual(await general(), initial, 'Reporting an issue must preserve General and its loaded page')
        await writeFile(resolve(directory, `general-${scheme}-${width}.png`), Buffer.from((await page.call('Page.captureScreenshot')).data, 'base64'))
      }
    }
    const reportedChanges = changes
    server.stdin.write(JSON.stringify({ status: 'recovered' }) + '\n')
    await until(async () => await attention() === undefined, 'Recovery must remove the issue without reloading General')
    assert.ok(changes > reportedChanges, 'Recovery must arrive through the real change stream')
    assert.deepEqual(await general(), initial, 'Recovery must preserve the General name, timezone and loaded page')
  } finally {
    if (chrome) {
      await chrome.close()
    }
    lines.close()
    server.stdin.end()
    await exited
  }
})
