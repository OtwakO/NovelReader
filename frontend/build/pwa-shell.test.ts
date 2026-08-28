import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { renderServiceWorker } from './pwa-shell';

const workerTemplate = `
const CACHE_NAME = 'novelreader-shell-__NOVELREADER_BUILD_ID__';
const SHELL_ASSETS = __NOVELREADER_SHELL_ASSETS__;
const isApiPath = (pathname) => pathname === '/api' || pathname.startsWith('/api/');
`;

describe('PWA shell build', () => {
  it('injects the build revision and complete shell asset manifest', () => {
    const source = renderServiceWorker(workerTemplate, 'revision-1', [
      '/',
      '/assets/app-123.js',
    ]);

    expect(source).toContain('novelreader-shell-revision-1');
    expect(source).toContain('["/","/assets/app-123.js"]');
    expect(source).not.toContain('__NOVELREADER_');
    expect(source).toContain("pathname === '/api'");
  });

  it('rejects a worker source without the build contract placeholders', () => {
    expect(() => renderServiceWorker('self.addEventListener("fetch", () => {})', 'revision-1', ['/']))
      .toThrow('PWA service-worker build placeholders are missing');
  });

  it('leaves orientation to the device and user settings', async () => {
    const manifest = JSON.parse(await readFile(resolve(process.cwd(), 'public/manifest.webmanifest'), 'utf8')) as Record<string, unknown>;
    expect(manifest).not.toHaveProperty('orientation');
  });
});
