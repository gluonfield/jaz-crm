import { AppBridge, PostMessageTransport } from '@modelcontextprotocol/ext-apps/app-bridge'

const frame = document.querySelector<HTMLIFrameElement>('iframe')!
const bridge = new AppBridge(null, { name: 'Synthetic host', version: '1' }, {
  serverTools: {}, openLinks: {}, experimental: { 'jaz/navigation': {} },
}, { hostContext: { theme: 'light', displayMode: 'fullscreen', 'jaz/navigation': { path: frame.dataset.path } } })
bridge.oncalltool = async ({ name, arguments: args }) => {
  const response = await fetch(`/api/tools/${name}`, { method: 'POST', body: JSON.stringify(args) })
  const structuredContent = await response.json()
  return { content: [], structuredContent, isError: !response.ok }
}
bridge.onopenlink = async () => ({})
await bridge.connect(new PostMessageTransport(frame.contentWindow!, frame.contentWindow!))
frame.srcdoc = await (await fetch('/app')).text()
