import test from 'node:test'
import assert from 'node:assert/strict'
import { spawn, execFileSync } from 'node:child_process'
import { createInterface } from 'node:readline'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { checkEmbedded } from './mcp-frame.mjs'
import { browser, click, until } from './browser.mjs'

const directory = resolve(process.env.SHORTCUT_OUTPUT ?? '../runs/shortcuts/browser')

test('CRM commands target real records, preserve typing and match menu actions', { timeout: 180000 }, async () => {
  await mkdir(directory, { recursive: true })
  const binary = resolve(directory, 'server')
  execFileSync('go', ['build', '-o', binary, './internal/httpapi/authapi/testdata/browser'], { cwd: resolve('..'), stdio: 'pipe' })
  const server = spawn(binary, [], { env: { ...process.env, SHORTCUT_WEB_DIR: process.env.SHORTCUT_WEB_DIR ?? resolve('dist/client') }, stdio: ['pipe', 'pipe', 'inherit'] })
  const exited = new Promise((accept) => server.once('close', accept))
  const lines = createInterface({ input: server.stdout })
  let chrome
  const results = []
  try {
    const config = await Promise.race([
      new Promise((accept, reject) => {
        lines.once('line', (line) => accept(JSON.parse(line)))
        server.once('exit', (code) => reject(new Error(`Fixture exited ${code}`)))
      }),
      new Promise((_, reject) => setTimeout(() => reject(new Error('Fixture startup timed out')), 20000).unref()),
    ])
    const tool = async (name, args = {}) => {
      const response = await fetch(`${config.url}/api/tools/${name}`, { method: 'POST', headers: { Cookie: `jc_session=${config.session}`, 'Content-Type': 'application/json' }, body: JSON.stringify(args) })
      const body = await response.json()
      assert.equal(response.ok, true, `${name}: ${JSON.stringify(body)}`)
      return body
    }
    await tool('create_object', { slug: 'examples', name: 'Examples' })
    await tool('create_attribute', { object:'examples', slug:'estimate', name:'Estimate', type:'number' })
    const records = []
    for (const object of ['people', 'companies', 'deals', 'pages', 'examples', 'follow_ups']) {
      for (let index = 0; index < 2; index++) {
        const { record } = await tool('upsert_record', { object, values: { name: `Synthetic ${object} ${index}`, ...(object === 'deals' && { stage: index ? 'Won' : 'Lead' }) } })
        records.push(record)
      }
    }
    chrome = await browser(directory)
    const page = await chrome.page('about:blank')
    await page.call('Emulation.setUserAgentOverride', { userAgent: 'Synthetic browser', platform: 'MacIntel' })
    await page.call('Emulation.setEmulatedMedia', { features: [{name:'prefers-color-scheme',value:'light'}] })
    await page.call('Runtime.enable')
    chrome.on('Runtime.exceptionThrown', (event) => console.error(JSON.stringify(event)))
    chrome.on('Page.javascriptDialogOpening', (event) => console.error(JSON.stringify(event)))
    await page.call('Network.setCookie', { name: 'jc_session', value: config.session, url: config.url })
    await page.call('Page.addScriptToEvaluateOnNewDocument', { source: `
      window.copied = []
      Object.defineProperty(navigator, 'clipboard', { value: {
        writeText: async (text) => {
          if (window.clipboardFailure) throw new Error('Synthetic clipboard denial')
          window.copied.push(text)
        },
      } })
    ` })
    const key = async (key, code = key, modifiers = 0) => {
      const windowsVirtualKeyCode = /^Key[A-Z]$/.test(code) ? code.charCodeAt(3) : /^Digit[0-9]$/.test(code) ? code.charCodeAt(5) : {Enter:13, Escape:27, Backspace:8, Period:190, Comma:188, Slash:191, Backslash:220}[code]
      await page.call('Input.dispatchKeyEvent', { type: 'keyDown', key, code, modifiers, windowsVirtualKeyCode, text: key === 'Enter' ? '\r' : undefined })
      await page.call('Input.dispatchKeyEvent', { type: 'keyUp', key, code, modifiers, windowsVirtualKeyCode })
    }
    const idle = () => page.evaluate('document.activeElement?.blur()')
    const navigate = async (path) => {
      await page.call('Page.resetNavigationHistory')
      const previous = await page.evaluate('performance.timeOrigin')
      await page.call('Page.navigate', { url: config.url + path })
      await until(() => page.evaluate(`performance.timeOrigin !== ${previous} && document.readyState === 'complete' && !!document.querySelector('main')`))
    }
    const recordPage = async (record) => {
      await navigate(`/r/${record.id}?tab=activity`)
      await until(() => page.evaluate(`document.querySelector('textarea[aria-label=Name]')?.value === ${JSON.stringify(record.values.name)}`))
      await idle()
    }
    const settle = (selector) => until(() => page.evaluate(`!document.querySelector(${JSON.stringify(selector)}).getAnimations({subtree:true}).some(animation => animation.playState === 'running')`))
    const copied = () => page.evaluate('window.copied.at(-1)')
    const menuItem = async (label) => {
      await page.evaluate(`Array.from(document.querySelectorAll('[role=menuitem]')).find(item => item.textContent.startsWith(${JSON.stringify(label)})).click()`)
      await until(() => page.evaluate('!document.querySelector("[role=menu]")'))
    }
    const check = async (name, run) => {
      if (process.env.SHORTCUT_CASE && !process.env.SHORTCUT_CASE.split(',').includes(name)) {
        return
      }
      try {
        await run()
        results.push({ name, passed: true })
      } catch (error) {
        results.push({ name, passed: false, error: error.message })
        const shot = await page.call('Page.captureScreenshot')
        await writeFile(resolve(directory, `${name}-failure.png`), Buffer.from(shot.data, 'base64'))
        await writeFile(resolve(directory, `${name}-failure.txt`), await page.evaluate('document.body.innerText'))

      }
      await writeFile(resolve(directory, 'results.json'), JSON.stringify(results, null, 2))
      assert.equal(results.at(-1).passed, true, `${name}: ${results.at(-1).error}`)
    }
    await check('copy_pages', async () => {
      for (const record of records.filter((_, index) => index % 2 === 0)) {
        await recordPage(record)
        await key('.', 'Period', 4)
        await until(async () => await copied() === record.id, 'Copy ID shortcut must preserve the record ID')
        assert.equal(await copied(), record.id, `Copy ID ${record.object}`)
        await key('<', 'Comma', 12)
        await until(async () => await copied() === `https://crm.jaz.chat/r/${record.id}`, 'Copy URL must be canonical')
        await click(page, '[aria-label=More]')
        await until(() => page.evaluate('!!document.querySelector("[role=menu]")'))
        assert.match(await page.evaluate('document.querySelector("[role=menu]").textContent'), /Copy ID.*⌘.*Copy link.*⌘/)
        if (record.object === 'examples') {
          await settle('[role=menu]')
          const shot = await page.call('Page.captureScreenshot')
          await writeFile(resolve(directory, 'menu-light.png'), Buffer.from(shot.data, 'base64'))
        }
        await menuItem('Copy ID')
        assert.equal(await copied(), record.id, `Menu ID ${record.object}`)
        await click(page, '[aria-label=More]')
        await until(() => page.evaluate('!!document.querySelector("[role=menu]")'))
        await menuItem('Copy link')
        assert.equal(await copied(), `https://crm.jaz.chat/r/${record.id}`, `Menu URL ${record.object}`)
        await until(() => page.evaluate('Array.from(document.querySelectorAll("[data-sonner-toast]")).some(node => node.textContent === "Copied")'))
      }
    })
    await check('typing', async () => {
      await recordPage(records[0])
      await page.evaluate('window.copied = []')
      await click(page, 'textarea[aria-label=Name]')
      for (const [value, code, modifiers] of [['.', 'Period', 4], ['<', 'Comma', 12], ['c', 'KeyC', 0], ['?', 'Slash', 8], ['k', 'KeyK', 4]]) {
        await key(value, code, modifiers)
      }
      assert.deepEqual(await page.evaluate('window.copied'), [], 'Typing must not copy')
      assert.equal(await page.evaluate('!!document.querySelector("[role=dialog]")'), false, 'Typing must not launch commands')
      await idle()
      await key('k', 'KeyK', 4)
      await until(() => page.evaluate('!!document.querySelector("[cmdk-input]")'))
      await key('k', 'KeyK', 4)
      await until(() => page.evaluate('!document.querySelector("[cmdk-input]")'))
    })
    await check('table_board', async () => {
      for (const path of ['/o/examples', '/o/deals?view=table', '/o/deals']) {
        await navigate(path)
        await until(() => page.evaluate(`!!document.querySelector('[data-row="0"]')`))
        await idle()
        await key('j', 'KeyJ')
        await key('.', 'Period', 4)
        const slug = path.includes('examples') ? 'examples' : 'deals'
        const id = await copied()
        assert.ok(records.some((record) => record.object === slug && record.id === id), 'Focused table/board copy must target its record')
        if (path === '/o/deals') {
          await key('ArrowRight', 'ArrowRight')
          await key('.', 'Period', 4)
          const right = await copied()
          assert.equal(records.find((record) => record.id === right)?.values.stage, 'Won', 'Right must select the next board column')
          await key('ArrowLeft', 'ArrowLeft')
          await key('.', 'Period', 4)
          assert.equal(await copied(), id, 'Left must return to the previous board column')
        }
        await key(path === '/o/examples' ? 'o' : 'Enter', path === '/o/examples' ? 'KeyO' : 'Enter')
        await until(() => page.evaluate(`location.pathname === '/r/${id}'`), 'Open must navigate to the selected record')
      }
    })
    await check('navigation_create_filter', async () => {
      await recordPage(records[0])
      for (const [value, destination] of [['i', '/triage'], ['m', '/o/follow_ups'], ['x', '/trash'], ['s', '/settings'], ['p', '/o/people'], ['c', '/o/companies']]) {
        await idle()
        await key('g', 'KeyG')
        await key(value, `Key${value.toUpperCase()}`)
        await until(() => page.evaluate(`location.pathname === '${destination}'`), 'Go-to shortcut must navigate')
      }
      await navigate('/o/examples')
      await until(() => page.evaluate(`!!document.querySelector('[data-row="0"]')`))
      await idle()
      await key('c', 'KeyC')
      await until(() => page.evaluate('!!document.querySelector("[role=dialog] input[aria-label=Name]")'), 'Create shortcut must open the form')
      await key('Escape', 'Escape')
      await until(() => page.evaluate('!document.querySelector("[role=dialog]")'))
      await idle()
      await key('f', 'KeyF')
      await until(() => page.evaluate('document.body.textContent.includes("Match all conditions")'))
    })
    await check('private_follow_up_filters', async () => {
      const title = (label) => page.evaluate(`Array.from(document.querySelectorAll('main button')).find(button => button.textContent.startsWith(${JSON.stringify(label)}))?.click()`)
      const button = (label) => page.evaluate(`Array.from(document.querySelectorAll('button')).find(button => button.textContent === ${JSON.stringify(label)})?.click()`)
      await navigate('/o/follow_ups')
      await until(() => page.evaluate(`Array.from(document.querySelectorAll('main button')).some(button => button.textContent.startsWith('Assigned to me'))`))
      await title('Assigned to me')
      await until(() => page.evaluate(`!!document.querySelector('[aria-label="Find saved filters…"]')`))
      assert.equal(await page.evaluate(`document.querySelectorAll('[cmdk-item]').length`), 2)
      await page.evaluate(`Array.from(document.querySelectorAll('[cmdk-item]')).find(item => item.textContent.startsWith('Everyone')).click()`)
      await until(() => page.evaluate(`new URLSearchParams(location.search).get('saved') === 'everyone'`))
      await click(page, '[aria-label=Filter]')
      await until(() => page.evaluate(`!!document.querySelector('[aria-label="Remove Status condition"]')`))
      await click(page, '[aria-label="Remove Status condition"]')
      assert.equal(await page.evaluate(`Array.from(document.querySelectorAll('button')).some(button => button.textContent === 'Save changes' || button.getAttribute('aria-label')?.startsWith('Delete saved filter'))`), false, 'Built-in views must have no update or delete action')
      await button('Save as…')
      await until(() => page.evaluate(`!!document.querySelector('input[aria-label="Filter name"]')`))
      assert.match(await page.evaluate(`document.querySelector('input[aria-label="Filter name"]').closest('[role=dialog]').textContent`), /Save private filter/)
      await page.call('Input.insertText', { text: 'Private browser view' })
      await button('Save')
      await until(async () => (await tool('list_saved_filters', { object: 'follow_ups' })).filters.length === 3)
      await until(() => page.evaluate(`Array.from(document.querySelectorAll('main button')).some(button => button.textContent.startsWith('Private browser view'))`))
      const selected = await page.evaluate('location.pathname + location.search')
      await navigate(selected)
      await until(() => page.evaluate(`Array.from(document.querySelectorAll('main button')).some(button => button.textContent.startsWith('Private browser view'))`))
      await click(page, '[aria-label=Filter]')
      await until(() => page.evaluate(`!!document.querySelector('[aria-label="Delete saved filter Private browser view"]')`))
      await click(page, '[aria-label="Delete saved filter Private browser view"]')
      await until(() => page.evaluate(`!!document.querySelector('[role=dialog]')`))
      await button('Delete')
      await until(() => page.evaluate(`Array.from(document.querySelectorAll('main button')).some(button => button.textContent.startsWith('Assigned to me'))`))
      assert.deepEqual((await tool('list_saved_filters', { object: 'follow_ups' })).filters.map(filter => filter.name), ['Everyone', 'Assigned to me'])
      await until(() => page.evaluate(`!document.querySelector('[role=dialog][data-state=closed]')`))
      await title('Assigned to me')
      await until(() => page.evaluate(`!!document.querySelector('[aria-label="Find saved filters…"]')`))
      await until(() => page.evaluate(`!document.querySelector('[aria-label="Find saved filters…"]').closest('[role=dialog]').getAnimations({ subtree: true }).some(animation => animation.playState === 'running')`))
      await writeFile(resolve(directory, 'follow-up-defaults-light.png'), Buffer.from((await page.call('Page.captureScreenshot')).data, 'base64'))
      await page.call('Emulation.setDeviceMetricsOverride', { width: 572, height: 900, deviceScaleFactor: 1, mobile: false })
      await page.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: 'dark' }] })
      assert.equal(await page.evaluate('document.documentElement.scrollWidth > innerWidth'), false)
      await writeFile(resolve(directory, 'follow-up-defaults-narrow-dark.png'), Buffer.from((await page.call('Page.captureScreenshot')).data, 'base64'))
      await page.call('Emulation.setDeviceMetricsOverride', { width: 1440, height: 900, deviceScaleFactor: 1, mobile: false })
      await page.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: 'light' }] })
    })
    await check('row_menus', async () => {
      for (const slug of ['people', 'companies', 'deals', 'pages', 'examples', 'follow_ups']) {
        await navigate(`/o/${slug}?view=table&group_by_conversation=false`)
        await until(() => page.evaluate(`!!document.querySelector('[data-row="0"] a[href^="/r/"]')`))
        const id = await page.evaluate(`document.querySelector('[data-row="0"] a[href^="/r/"]').getAttribute('href').split('/').at(-1)`)
        const point = await page.evaluate(`(() => {
          const box = document.querySelector('[data-row="0"]').getBoundingClientRect()
          return {x: box.x + 100, y: box.y + box.height / 2}
        })()`)
        for (const [label, expected] of [['Copy ID', id], ['Copy link', `https://crm.jaz.chat/r/${id}`]]) {
          await page.call('Input.dispatchMouseEvent', {type:'mousePressed', button:'right', clickCount:1, ...point})
          await page.call('Input.dispatchMouseEvent', {type:'mouseReleased', button:'right', clickCount:1, ...point})
          await until(() => page.evaluate('!!document.querySelector("[role=menu]")'))
          await menuItem(label)
          assert.equal(await copied(), expected, `Row menu ${slug} ${label}`)
        }
      }
    })
    await check('property_save', async () => {
      const record = records.find((record) => record.object === 'deals')
      await recordPage(record)
      await key('s', 'KeyS')
      await until(() => page.evaluate('!!document.querySelector("[cmdk-input]")'), 'Status picker did not open')
      await key('k', 'KeyK', 4)
      assert.equal(await page.evaluate(`!!document.querySelector('[data-command-palette]')`), false, 'Property picker must keep typing keys')
      await page.evaluate(`Array.from(document.querySelectorAll('[cmdk-item]')).find(item => item.textContent.includes('Won')).click()`)
      await until(async () => (await tool('get_record', {record_id: record.id})).values.stage === 'Won', 'Status command must persist')
      assert.equal((await tool('get_record', {record_id: record.id})).values.stage, 'Won', 'Status command must persist')
      await until(() => page.evaluate('!document.querySelector("[role=dialog]")'), 'Property dialog must close')
      await idle()
      await key('¡', 'Digit1', 5)
      await until(async () => (await tool('get_record', {record_id:record.id})).values.stage === 'Lead', 'Numbered status must persist')
      await tool('upsert_record', {object:'deals', record_id:record.id, remove:{owner:[]}})
      assert.equal((await tool('get_record', {record_id:record.id})).values.owner, undefined)
      await key('i', 'KeyI')
      const workspace = await tool('get_workspace')
      const me = workspace.members.find((member) => member.is_me)
      await until(async () => (await tool('get_record', {record_id: record.id})).values.owner === me.email, 'Assign to me must persist')
      const followup = records.find((record) => record.object === 'follow_ups')
      await tool('save_action_date', {record_id:followup.id, value:'2026-12-01'})
      await recordPage(followup)
      await key('D', 'KeyD', 12)
      await until(async () => !(await tool('get_record', {record_id:followup.id})).values.action_date, 'Clear date must use the date API')
      assert.equal((await tool('get_record', {record_id:followup.id})).values.action_date, undefined, 'Clear date must use the date API')
      const example = records.find((record) => record.object === 'examples')
      await recordPage(example)
      await key('E', 'KeyE', 8)
      await until(() => page.evaluate('document.activeElement?.getAttribute("type") === "number"'), 'Numeric property must receive focus')
      await page.call('Input.insertText', {text:'7'})
      await idle()
      await until(async () => (await tool('get_record', {record_id:example.id})).values.estimate === '7', 'Estimate shortcut must persist')
    })
    await check('create_delete_restore', async () => {
      await navigate('/o/examples')
      await until(() => page.evaluate(`!!document.querySelector('[data-row="0"]')`))
      await idle()
      await key('c', 'KeyC')
      await until(() => page.evaluate('!!document.querySelector("[role=dialog] input[aria-label=Name]")'), 'Create shortcut must open the form')
      await page.call('Input.insertText', {text:'Created by keyboard'})
      await key('Enter', 'Enter', 4)
      await until(() => page.evaluate(`location.pathname.startsWith('/r/') && document.querySelector('textarea[aria-label=Name]')?.value === 'Created by keyboard'`))
      const id = await page.evaluate("location.pathname.split('/').at(-1)")
      assert.equal((await tool('get_record', {record_id:id})).values.name, 'Created by keyboard', 'Create must save')
      await idle()
      await key('Backspace', 'Backspace', 4)
      await until(() => page.evaluate('!!document.querySelector("[role=dialog]")'))
      await page.evaluate(`Array.from(document.querySelectorAll('[role=dialog] button')).find(button => button.textContent === 'Delete').click()`)
      await until(async () => (await tool('list_trash')).records.some((record) => record.id === id))
      await navigate('/trash')
      await until(() => page.evaluate(`!!document.querySelector('[data-row="0"]')`))
      await idle()
      await key('j', 'KeyJ')
      await key('#', 'Digit3', 8)
      await until(async () => !(await tool('list_trash')).records.some((record) => record.id === id), 'Restore shortcut must restore its record')
      assert.equal((await tool('get_record', {record_id:id})).values.name, 'Created by keyboard', 'Restore must save')
    })
    await check('editor_and_failure', async () => {
      const record = records.find((record) => record.object === 'pages')
      await recordPage(record)
      await click(page, '[contenteditable=true]')
      await page.evaluate('window.copied = []')
      await key('.', 'Period', 4)
      await key('?', 'Slash', 8)
      assert.deepEqual(await page.evaluate('window.copied'), [], 'Editor must not copy')
      assert.equal(await page.evaluate('!!document.querySelector("[role=dialog]")'), false, 'Editor must keep its own keys')
      await page.call('Input.insertText', {text:'A todo'})
      await key('&', 'Digit7', 12)
      await until(() => page.evaluate('!!document.querySelector("[data-type=taskList]")'), 'Todo shortcut must format the document')
      await idle()
      await until(async () => (await tool('get_record', {record_id:record.id})).values.content?.includes('- [ ] A todo'), 'Editor shortcut must persist Markdown')
      await page.evaluate('window.clipboardFailure = true')
      await key('.', 'Period', 4)
      await until(() => page.evaluate('document.body.textContent.includes("Unable to copy to clipboard")'), 'Clipboard denial must show its error')
      assert.deepEqual(await page.evaluate('window.copied'), [], 'Failed clipboard must not claim success')
    })
    await check('native_control', async () => {
      await navigate('/o/examples')
      await until(() => page.evaluate(`!!document.querySelector('[data-row="0"]')`))
      await idle()
      await key('j', 'KeyJ')
      await page.evaluate(`Array.from(document.querySelectorAll('main button')).find(button => button.textContent.trim() === 'New').focus()`)
      await key('Enter', 'Enter')
      await until(() => page.evaluate('!!document.querySelector("[role=dialog] input[aria-label=Name]")'), 'Native Enter must activate its focused button')
      await page.evaluate('window.copied = []')
      await key('.', 'Period', 4)
      assert.deepEqual(await page.evaluate('window.copied'), [], 'Dialog input must not copy')
    })
    await check('sequence_platform_palette', async () => {
      await recordPage(records[0])
      await key('g', 'KeyG')
      await page.evaluate('document.querySelector("textarea[aria-label=Name]").focus()')
      await idle()
      await key('x', 'KeyX')
      assert.equal(await page.evaluate('location.pathname'), `/r/${records[0].id}`, 'Focus must cancel a go sequence')
      await key('g', 'KeyG')
      await page.call('Input.dispatchMouseEvent', {type:'mousePressed', button:'left', clickCount:1, x:1000, y:850})
      await page.call('Input.dispatchMouseEvent', {type:'mouseReleased', button:'left', clickCount:1, x:1000, y:850})
      await key('x', 'KeyX')
      assert.equal(await page.evaluate('location.pathname'), `/r/${records[0].id}`, 'Pointer must cancel a go sequence')
      await page.call('Emulation.setUserAgentOverride', {userAgent:'Synthetic browser', platform:'Linux x86_64'})
      await page.evaluate('window.copied = []')
      await key('.', 'Period', 4)
      assert.deepEqual(await page.evaluate('window.copied'), [], 'Linux Meta must not act as Ctrl')
      await key('.', 'Period', 2)
      await until(async () => await copied() === records[0].id)
      await key('k', 'KeyK', 2)
      await until(() => page.evaluate('!!document.querySelector("[cmdk-input]")'))
      await page.call('Input.insertText', {text:'Copy record URL'})
      await until(() => page.evaluate(`!!document.querySelector('[cmdk-item][data-value="command:copyUrl"]')`))
      await key('Enter', 'Enter')
      await until(async () => await copied() === `https://crm.jaz.chat/r/${records[0].id}`)
      await until(() => page.evaluate('!document.querySelector("[role=dialog]")'), 'Palette action must close')
      await page.call('Emulation.setUserAgentOverride', {userAgent:'Synthetic browser', platform:'MacIntel'})
    })
    await check('embedded', () => checkEmbedded(chrome, directory, config, records[0]))
    await check('help', async () => {
      await page.call('Page.bringToFront')
      await recordPage(records[0])
      for (const scheme of ['light', 'dark']) {
        await page.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: scheme }] })
        for (const width of [1440, 390]) {
          await page.call('Emulation.setDeviceMetricsOverride', { width, height: 900, deviceScaleFactor: 1, mobile: false })
          await idle()
          await key('?', 'Slash', 8)
          await until(() => page.evaluate('!!document.querySelector("[role=dialog]")'), 'Help shortcut must open the panel')
          assert.match(await page.evaluate('document.querySelector("[role=dialog]").textContent'), /Keyboard Shortcuts.*Copy record ID.*Copy record URL/)
          assert.equal(await page.evaluate('document.documentElement.scrollWidth <= innerWidth'), true, 'Help must fit narrow width')
          await settle('[role=dialog]')
          const shot = await page.call('Page.captureScreenshot')
          await writeFile(resolve(directory, `help-${scheme}-${width}.png`), Buffer.from(shot.data, 'base64'))
          assert.equal(await page.evaluate(`(() => {
            const panel = document.querySelector('[role=dialog]')
            const scroll = panel.querySelector('.overflow-y-auto')
            scroll.scrollTop = scroll.scrollHeight
            return scroll.lastElementChild.getBoundingClientRect().bottom <= panel.getBoundingClientRect().bottom
          })()`), true, 'All help groups must be reachable')
          await key('Escape', 'Escape')
          await until(() => page.evaluate('!document.querySelector("[role=dialog]")'))
        }
      }
    })
  } finally {
    if (chrome) {
      await chrome.close()
    }
    lines.close()
    server.stdin.end()
    await exited
  }
})
