import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['icon.svg'],
      manifest: {
        name: 'BarberFlow',
        short_name: 'BarberFlow',
        description: 'Agenda inteligente para sua barbearia',
        theme_color: '#111827',
        background_color: '#f7f4ee',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: '/icon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any maskable' }
        ]
      },
      workbox: {
        navigateFallback: '/index.html',
        runtimeCaching: [{
          urlPattern: ({ url }) => url.pathname.startsWith('/api/'),
          handler: 'NetworkOnly'
        }]
      }
    })
  ],
  server: {
    port: 5173,
    proxy: { '/api': { target: 'http://api:8080', changeOrigin: true } }
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: false,
    // Vitest's default include glob also matches e2e/*.spec.ts, but those
    // use @playwright/test's test/expect (run via `npm run test:e2e`), not
    // vitest's - keep this to src/ so the two suites don't collide.
    include: ['src/**/*.test.{ts,tsx}']
  }
})
