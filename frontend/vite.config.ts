import {defineConfig} from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import {VitePWA} from 'vite-plugin-pwa';

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      // 'prompt' (not the design doc's literal 'autoUpdate') so the SW exposes
      // needRefresh and the reload toast (Task 2) actually fires — autoUpdate
      // silently swaps the SW and never triggers onNeedRefresh, leaving the
      // toast dead. This honors the design doc's intent (a reload toast).
      registerType: 'prompt',
      // Icons are deferred until artwork lands; the manifest is otherwise
      // complete. Add 192/512 maskable PNG entries here to make the app
      // installable (see Task 7).
      manifest: {
        name: 'BandWidth',
        short_name: 'BandWidth',
        description: 'Practice tracking for musicians and bands',
        theme_color: '#1d4ed8',
        background_color: '#ffffff',
        display: 'standalone',
        start_url: '/',
        icons: [],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,woff2}'],
        // v1: never serve stale API data — the SPA always hits the network
        // for /api and shows its own loading/error states.
        runtimeCaching: [
          {
            urlPattern: ({url}) => url.pathname.startsWith('/api'),
            handler: 'NetworkOnly',
          },
        ],
        // SPA deep links fall back to index.html (matches RegisterSPA on the
        // server), but never for /api requests.
        navigateFallback: '/index.html',
        navigateFallbackDenylist: [/^\/api/],
      },
    }),
  ],
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
    },
  },
});
