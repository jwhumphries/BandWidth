import react from '@vitejs/plugin-react';
import {defineConfig} from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    testTimeout: 15000,
    alias: {
      'virtual:pwa-register/react': new URL(
        './src/test/pwa-register-stub.ts',
        import.meta.url,
      ).pathname,
    },
  },
});
