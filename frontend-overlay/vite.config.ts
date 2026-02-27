import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  // Base path for asset URLs — SaaS frontend assets are served under /last/
  // so they don't conflict with the main llmopt frontend's /assets/ directory.
  base: '/last/',
  plugins: [react(), tailwindcss()],
  server: {
    port: parseInt(process.env.VITE_PORT || '4280'),
    proxy: {
      '/api': {
        target: process.env.VITE_API_URL || 'http://localhost:4290',
        changeOrigin: true,
      },
    },
  },
})
