import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { execFileSync } from 'node:child_process'
import { readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { click, until } from './browser.mjs'

export async function checkEmbedded(chrome, directory, config, record) {
  const bundle = resolve(directory, 'host.js')
  execFileSync('bun', ['build', 'tests/mcp-host.ts', '--target', 'browser', '--outfile', bundle], { stdio: 'pipe' })
  const host = await readFile(bundle)
  const html = (await readFile(process.env.SHORTCUT_MCP_HTML ?? resolve('../backend/internal/httpapi/mcpapi/app/mcp-app.html'), 'utf8'))
    .replace('<div id="root"', `<div data-start-path="/r/${record.id}" id="root"`)
    .replace('<head>', `<head><script>document.documentElement.dataset.clipboardAvailable = String(typeof navigator.clipboard?.writeText === 'function')
Object.defineProperty(navigator, 'clipboard', {value:{writeText:async(text)=>parent.postMessage({copied:text},'*')}})</script>`)
  const server = createServer(async (req, res) => {
    if (req.url === '/host.js') {
      res.setHeader('Content-Type', 'text/javascript')
      res.end(host)
    } else if (req.url === '/app') {
      res.end(html)
    } else if (req.url.startsWith('/api/tools/')) {
      let body = ''
      for await (const chunk of req) {
        body += chunk
      }
      const response = await fetch(config.url + req.url, { method:'POST', headers:{ Cookie:`jc_session=${config.session}`, 'Content-Type':'application/json' }, body })
      res.writeHead(response.status, { 'Content-Type':'application/json' })
      res.end(await response.text())
    } else {
      res.setHeader('Content-Type', 'text/html')
      res.end(`<meta charset="utf-8"><style>body{margin:0}iframe{width:100vw;height:100vh;border:0}</style><iframe title="CRM" data-path="/r/${record.id}" sandbox="allow-scripts allow-forms allow-popups" allow="clipboard-write"></iframe><script>window.copies = []
addEventListener('message', (event) => {
  if (event.data.copied) {
    window.copies.push(event.data.copied)
  }
})</script><script type="module" src="/host.js"></script>`)
    }
  })
  await new Promise((accept) => server.listen(0, '127.0.0.1', accept))
  let page
  try {
    page = await chrome.page(`http://127.0.0.1:${server.address().port}`)
    await page.call('Runtime.enable')
    chrome.on('Runtime.consoleAPICalled', (event) => {
      if (event.type === 'error') {
        console.error(JSON.stringify(event.args))
      }
    })
    let evaluate
    await until(async () => {
      const { targetInfos } = await chrome.send('Target.getTargets')
      const target = targetInfos.find((target) => target.type === 'iframe' && target.url === 'about:srcdoc')
      if (target) {
        const { sessionId } = await chrome.send('Target.attachToTarget', {targetId:target.targetId, flatten:true})
        await chrome.send('Runtime.enable', {}, sessionId)
        evaluate = async (expression) => (await chrome.send('Runtime.evaluate', {expression, returnByValue:true}, sessionId)).result.value
        return true
      }
      const { frameTree } = await page.call('Page.getFrameTree')
      const child = frameTree.childFrames?.find((entry) => entry.frame.url === 'about:srcdoc')?.frame
      if (child) {
        const { executionContextId } = await page.call('Page.createIsolatedWorld', {frameId:child.id, worldName:'verification'})
        evaluate = async (expression) => (await page.call('Runtime.evaluate', {expression, contextId:executionContextId, returnByValue:true})).result.value
        return true
      }
      return false
    }, 'Embedded frame did not navigate')
    try {
      await until(() => evaluate(`!!document.querySelector('[aria-label=More]')`), 'Embedded record did not load')
    } catch (error) {
      await writeFile(resolve(directory, 'mcp-dom.txt'), String(await evaluate('document.documentElement.outerHTML')))
      throw error
    }
    await writeFile(resolve(directory, 'mcp-clipboard-api.txt'), String(await evaluate('document.documentElement.dataset.clipboardAvailable')))
    await click(page, 'iframe')
    await evaluate('document.activeElement?.blur()')
    for (const [key, code, modifiers, expected] of [['.', 'Period', 4, record.id], ['<', 'Comma', 12, `https://crm.jaz.chat/r/${record.id}`]]) {
      await page.call('Input.dispatchKeyEvent', {type:'keyDown', key, code, modifiers})
      await page.call('Input.dispatchKeyEvent', {type:'keyUp', key, code, modifiers})
      await until(() => page.evaluate(`window.copies.at(-1) === ${JSON.stringify(expected)}`), 'Embedded copy must reach the clipboard')
      assert.equal(await page.evaluate('window.copies.at(-1)'), expected, 'Embedded copy must use the record identity')
    }
    await page.call('Input.dispatchKeyEvent', {type:'keyDown', key:'?', code:'Slash', modifiers:8})
    await until(() => evaluate(`document.querySelector('[role=dialog]')?.textContent.includes('Keyboard Shortcuts')`))
  } finally {
    if (page) {
      await chrome.send('Target.closeTarget', {targetId:page.targetId})
    }
    server.closeAllConnections()
    await new Promise((accept) => server.close(accept))
  }
}
