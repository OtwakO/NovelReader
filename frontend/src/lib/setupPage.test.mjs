import { readFile } from 'node:fs/promises';
import test from 'node:test';
import assert from 'node:assert/strict';
import { compile } from 'svelte/compiler';

test('setup page compiles in Svelte 5 runes mode', async () => {
  const filename = new URL('./SetupPage.svelte', import.meta.url);
  const source = await readFile(filename, 'utf8');
  const result = compile(source, {
    filename: filename.pathname,
    generate: 'client',
    modernAst: true,
  });
  assert.equal(result.warnings.length, 0);
});
