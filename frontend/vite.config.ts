import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { chunkBudgets } from './build/chunk-budgets.js';
import { pwaShell } from './build/pwa-shell.js';

export default defineConfig({
  // Native select markup; src/ui/README.md records the Vue nesting-validator workaround.
  plugins: [vue({ template: { compilerOptions: { isCustomElement: tag => tag === 'selectedcontent' } } }), pwaShell(), chunkBudgets()],
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
