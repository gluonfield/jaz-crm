import tailwindcss from '@tailwindcss/vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const api = process.env.JAZ_CRM_API ?? 'http://localhost:7500'

export default defineConfig({
  resolve: { tsconfigPaths: true },
  server: {
    port: 7501,
    strictPort: true,
    proxy: Object.fromEntries(['/api', '/auth', '/connections/google', '/mcp', '/oauth', '/.well-known', '/favicon.svg', '/page-icons'].map((path) => [path, api])),
  },
  plugins: [tailwindcss(), tanstackStart({ spa: { enabled: true, prerender: { outputPath: '/index.html' } } }), viteReact()],
})
