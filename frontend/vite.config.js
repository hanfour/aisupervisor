import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../cmd/aisupervisor-gui/frontend/dist',
    emptyOutDir: true,
  },
})
