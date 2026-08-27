import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import type { Plugin } from 'vite';
const buildIdPlaceholder = '__NOVELREADER_BUILD_ID__';
const assetsPlaceholder = '__NOVELREADER_SHELL_ASSETS__';
const publicShellFiles = [
  'manifest.webmanifest',
  'icons/icon-192.png',
  'icons/icon-512.png',
  'icons/icon-maskable-512.png',
];
const publicShellAssets = ['/', '/index.html', ...publicShellFiles.map((path) => `/${path}`)];

type BundleOutput = {
  fileName: string;
  type: 'asset';
  source: string | Uint8Array;
} | {
  fileName: string;
  type: 'chunk';
  code: string;
};

function buildRevision(outputs: BundleOutput[], additionalContent: Array<string | Uint8Array>): string {
  const hash = createHash('sha256');
  for (const output of outputs) {
    hash.update(output.fileName);
    hash.update('\0');
    hash.update(output.type === 'chunk' ? output.code : output.source);
    hash.update('\0');
  }
  for (const content of additionalContent) {
    hash.update(content);
    hash.update('\0');
  }
  return hash.digest('hex').slice(0, 16);
}

export function renderServiceWorker(workerSource: string, buildId: string, shellAssets: string[]): string {
  if (!workerSource.includes(buildIdPlaceholder) || !workerSource.includes(assetsPlaceholder)) {
    throw new Error('PWA service-worker build placeholders are missing');
  }
  return workerSource
    .replace(buildIdPlaceholder, buildId)
    .replace(assetsPlaceholder, JSON.stringify(shellAssets));
}

export function pwaShell(): Plugin {
  let projectRoot = '';

  return {
    name: 'novelreader-pwa-shell',
    apply: 'build',
    configResolved(config) {
      projectRoot = config.root;
    },
    async generateBundle(_options, bundle) {
      const outputs = Object.values(bundle).sort((left, right) => left.fileName.localeCompare(right.fileName));
      const shellAssets = [...new Set([
        ...publicShellAssets,
        ...outputs.map((output) => `/${output.fileName}`),
      ])];
      const workerSource = await readFile(resolve(projectRoot, 'src/pwa/service-worker.js'), 'utf8');
      const publicContent = await Promise.all(
        publicShellFiles.map((path) => readFile(resolve(projectRoot, 'public', path))),
      );

      this.emitFile({
        type: 'asset',
        fileName: 'service-worker.js',
        source: renderServiceWorker(
          workerSource,
          buildRevision(outputs, [workerSource, ...publicContent]),
          shellAssets,
        ),
      });
    },
  };
}
