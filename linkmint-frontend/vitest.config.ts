/**
 * Vitest config for the component kit (work03). jsdom + the React plugin render components under
 * the real Ivory theme; `setupFiles` registers jest-dom matchers and the jsdom shims Chakra's
 * positioners need. The `@` alias mirrors tsconfig `paths` (Vite does not read tsconfig paths and
 * `vite-tsconfig-paths` is not installed), so `@/theme/system` resolves in tests.
 */

import { fileURLToPath } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./vitest.setup.ts'],
    css: false,
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
});
