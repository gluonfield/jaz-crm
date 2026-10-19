import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { mkdtemp, rm } from 'node:fs/promises'
import { resolve } from 'node:path'

export async function browser(directory) {
  const executable = process.env.CHROME ?? ['/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', '/usr/bin/chromium', '/usr/bin/google-chrome'].find(existsSync)
  if (!executable) {
    throw new Error('Set CHROME to a Chrome/Chromium binary to run the headless interaction tests')
  }
  const profile = await mkdtemp(resolve(directory, 'browser-'))
  const processHandle = spawn(executable, ['--headless=new', '--no-first-run', '--no-default-browser-check', '--disable-background-networking', '--disable-features=BackForwardCache', '--disable-extensions', '--remote-debugging-pipe', `--user-data-dir=${profile}`], { stdio: ['ignore', 'ignore', 'inherit', 'pipe', 'pipe'] })
  const exited = new Promise((accept) => processHandle.once('close', accept))
  let closing = false
  let failure
  let seq = 0
  let buffer = ''
  const pending = new Map()
  const listeners = new Map()
  const failed = (error) => {
    if (closing) {
      return
    }
    failure = error
    for (const callback of pending.values()) {
      callback({ error: { message: error.message } })
    }
    pending.clear()
  }
  processHandle.on('error', failed)
  processHandle.on('exit', (code, signal) => failed(new Error(`Browser exited: ${signal ?? code}`)))
  processHandle.stdio[3].on('error', failed)
  processHandle.stdio[4].on('error', failed)
  processHandle.stdio[4].on('data', (chunk) => {
    buffer += chunk.toString()
    let end
    while ((end = buffer.indexOf('\0')) >= 0) {
      const message = JSON.parse(buffer.slice(0, end))
      buffer = buffer.slice(end + 1)
      if (message.id) {
        const callback = pending.get(message.id)
        pending.delete(message.id)
        callback?.(message)
      } else {
        listeners.get(message.method)?.(message.params, message.sessionId)
      }
    }
  })
  const send = (method, params = {}, sessionId) => new Promise((accept, reject) => {
    if (failure || closing) {
      reject(failure ?? new Error('Browser is closed'))
      return
    }
    const id = ++seq
    const timer = setTimeout(() => {
      pending.delete(id)
      reject(new Error(`Timed out: ${method}`))
    }, 10000)
    pending.set(id, (message) => {
      clearTimeout(timer)
      if (message.error) {
        reject(new Error(JSON.stringify(message.error)))
      } else {
        accept(message.result)
      }
    })
    processHandle.stdio[3].write(JSON.stringify({ id, method, params, sessionId }) + '\0')
  })
  return {
    send,
    on: (method, callback) => listeners.set(method, callback),
    async page(url) {
      const { targetId } = await send('Target.createTarget', { url: 'about:blank' })
      const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true })
      const call = (method, params) => send(method, params, sessionId)
      await call('Page.enable')
      await call('Network.enable')
      await call('Network.setCacheDisabled', { cacheDisabled: true })
      await call('Emulation.setDeviceMetricsOverride', { width: 1440, height: 900, deviceScaleFactor: 1, mobile: false })
      await call('Page.navigate', { url })
      return {
        targetId,
        call,
        async evaluate(expression) {
          const result = await call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
          if (result.exceptionDetails) {
            throw new Error(result.exceptionDetails.exception?.description ?? result.exceptionDetails.text)
          }
          return result.result.value
        },
      }
    },
    async close() {
      closing = true
      processHandle.kill('SIGKILL')
      await exited
      await rm(profile, { recursive: true, force: true })
    },
  }
}

export async function until(check, message = 'Timed out waiting for browser state') {
  const end = Date.now() + 10000
  while (Date.now() < end) {
    if (await check()) {
      return
    }
    await new Promise((accept) => setTimeout(accept, 30))
  }
  throw new Error(message)
}

export async function click(page, selector) {
  const point = await page.evaluate(`(() => {
    const rect = document.querySelector(${JSON.stringify(selector)}).getBoundingClientRect()
    return { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 }
  })()`)
  await page.call('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', clickCount: 1, ...point })
  await page.call('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'left', clickCount: 1, ...point })
}
