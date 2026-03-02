import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 41229,
  },
  build: {
    outDir: '../cmd/aisupervisor-gui/frontend/dist',
    emptyOutDir: true,
    assetsInlineLimit: 100000, // inline all assets < 100KB as base64 (body.png is 25KB)
  },
})
