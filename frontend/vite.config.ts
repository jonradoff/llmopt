import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 4280,
    proxy: {
      // LastSaaS API routes → lastsaas backend
      '/api/auth': 'http://localhost:4291',
      '/api/users': 'http://localhost:4291',
      '/api/tenants': 'http://localhost:4291',
      '/api/tenant': 'http://localhost:4291',
      '/api/billing': 'http://localhost:4291',
      '/api/plans': 'http://localhost:4291',
      '/api/messages': 'http://localhost:4291',
      '/api/admin': 'http://localhost:4291',
      '/api/config': 'http://localhost:4291',
      '/api/branding': 'http://localhost:4291',
      '/api/bundles': 'http://localhost:4291',
      '/api/stripe': 'http://localhost:4291',
      '/api/webhooks': 'http://localhost:4291',
      '/api/bootstrap': 'http://localhost:4291',
      '/api/credit-bundles': 'http://localhost:4291',
      '/api/announcements': 'http://localhost:4291',
      '/api/usage': 'http://localhost:4291',
      '/api/docs': 'http://localhost:4291',
      // All other /api routes → llmopt backend
      '/api': 'http://localhost:4281',
    },
  },
})
