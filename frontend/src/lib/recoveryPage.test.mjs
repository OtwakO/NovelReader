import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import { compile } from 'svelte/compiler';

test('RecoveryPage compiles with the configured Svelte compiler', () => {
  const filename = new URL('./RecoveryPage.svelte', import.meta.url);
  const source = fs.readFileSync(filename, 'utf8');
  const result = compile(source, { filename: filename.pathname, generate: 'client' });
  assert.equal(result.warnings.length, 0);
  assert.match(result.js.code, /recoverAdministrator/);
});
