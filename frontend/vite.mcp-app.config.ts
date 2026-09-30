import tailwindcss from '@tailwindcss/vite'
import viteReact from '@vitejs/plugin-react'
import { type Plugin, defineConfig } from 'vite'
import { viteSingleFile } from 'vite-plugin-singlefile'

// The document shell links the stylesheet by URL; the MCP App renders no
// shell and inlines the stylesheet instead, so that link resolves to nothing.
const noShellStylesheet: Plugin = {
  name: 'no-shell-stylesheet',
  enforce: 'pre',
  resolveId: (id) => (id.endsWith('styles.css?url') ? '\0shell-stylesheet' : undefined),
  load: (id) => (id === '\0shell-stylesheet' ? 'export default ""' : undefined),
}

// Builds the MCP App resource: one self-contained HTML file, fonts included,
// written next to the Go package that embeds it.
export default defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [noShellStylesheet, tailwindcss(), viteReact(), viteSingleFile()],
  build: {
    outDir: '../backend/internal/httpapi/mcpapi/app',
    emptyOutDir: false,
    assetsInlineLimit: Number.MAX_SAFE_INTEGER,
    rollupOptions: { input: 'mcp-app.html' },
  },
})
