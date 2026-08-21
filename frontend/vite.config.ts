import { defineConfig, type Plugin } from 'vite';
import vue from '@vitejs/plugin-vue';

const normalChunkBudgetKiB = 500;
const traditionalConversionBudgetKiB = 1120;

function chunkBudgets(): Plugin {
  return {
    name: 'novelreader-chunk-budgets',
    generateBundle(_options, bundle) {
      for (const output of Object.values(bundle)) {
        if (output.type !== 'chunk') continue;
        const isTraditionalConversion = Object.keys(output.modules).some((id) => id.includes('/opencc-js/dist/esm/cn2t.js'));
        const budgetKiB = isTraditionalConversion ? traditionalConversionBudgetKiB : normalChunkBudgetKiB;
        const sizeKiB = new TextEncoder().encode(output.code).byteLength / 1024;
        if (sizeKiB > budgetKiB) {
          this.error(`${output.fileName} is ${sizeKiB.toFixed(1)} KiB and exceeds its ${budgetKiB} KiB bundle budget.`);
        }
      }
    },
  };
}

export default defineConfig({
  plugins: [vue(), chunkBudgets()],
  build: {
    // OpenCC's lazy generic Traditional phrase dictionary is intentionally larger;
    // chunkBudgets still enforces 500 KiB for every other JavaScript chunk.
    chunkSizeWarningLimit: traditionalConversionBudgetKiB,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8888',
    },
  },
});
