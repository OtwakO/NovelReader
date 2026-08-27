import type { Plugin } from 'vite';

const defaultChunkBudgetBytes = 500 * 1024;
const traditionalDictionaryBudgetBytes = 1120 * 1024;

export function chunkBudgets(): Plugin {
  return {
    name: 'chunk-budgets',
    generateBundle(_options, bundle) {
      for (const output of Object.values(bundle)) {
        if (output.type !== 'chunk') continue;
        const bytes = new TextEncoder().encode(output.code).byteLength;
        const budget = output.fileName.includes('cn2t')
          ? traditionalDictionaryBudgetBytes
          : defaultChunkBudgetBytes;
        if (bytes > budget) {
          this.error(`${output.fileName} is ${(bytes / 1024).toFixed(1)} KiB; budget is ${budget / 1024} KiB`);
        }
      }
    },
  };
}
