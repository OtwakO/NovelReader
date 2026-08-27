import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { chunkBudgets } from './build/chunk-budgets.js';
import { pwaShell } from './build/pwa-shell.js';

export default defineConfig({
  plugins: [vue(), pwaShell(), chunkBudgets()],
  build: {
    chunkSizeWarningLimit: 1120,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('opencc-js/dist/esm/cn2t')) return 'cn2t';
          if (id.includes('opencc-js/dist/esm/t2cn')) return 't2cn';
          return undefined;
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8888',
    },
  },
});
