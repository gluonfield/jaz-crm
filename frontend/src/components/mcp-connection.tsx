import { Button } from '@jaz/ui/button'
import { useQuery } from '@tanstack/react-query'
import { Copy } from 'lucide-react'
import { toast } from 'sonner'

export function McpConnection() {
  const { data: url, isError } = useQuery({
    queryKey: ['mcp-url'],
    queryFn: async () => {
      const embeddedURL = document.getElementById('root')?.dataset.mcpUrl
      if (embeddedURL) {
        return embeddedURL
      }
      const response = await fetch('/.well-known/oauth-protected-resource/mcp')
      if (!response.ok) {
        throw new Error('Unable to load MCP URL')
      }
      const metadata: { resource: string } = await response.json()
      return metadata.resource
    },
    staleTime: Infinity,
  })
  return (
    <div className="px-4 py-3">
      <p className="mb-2 text-[12.5px] text-ink-3">Connect your agent using this URL and sign in when prompted.</p>
      <div className="flex items-center gap-3">
        <code className="min-w-0 flex-1 select-all break-all font-mono text-[12px] text-ink">{url ?? (isError ? 'Unable to load MCP URL' : '…')}</code>
        <Button aria-label="Copy MCP URL" disabled={!url} onClick={async () => {
          try {
            await navigator.clipboard.writeText(url!)
            toast.success('MCP URL copied')
          } catch {
            toast.error('Unable to copy. Select the URL to copy it.')
          }
        }}><Copy />Copy</Button>
      </div>
    </div>
  )
}
