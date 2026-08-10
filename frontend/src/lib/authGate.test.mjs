import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { compile } from 'svelte/compiler';

const root = path.resolve(import.meta.dirname, '..');

for (const file of ['App.svelte', 'lib/LoginPage.svelte', 'lib/RegistrationPage.svelte', 'lib/AuthenticatedApp.svelte']) {
  test(`${file} compiles`, () => {
    const source = fs.readFileSync(path.join(root, file), 'utf8');
    const result = compile(source, { filename: file, generate: 'client' });
    assert.ok(result.js.code.length > 0);
  });
}

test('root gate resolves setup and account before private app mount', () => {
  const source = fs.readFileSync(path.join(root, 'App.svelte'), 'utf8');
  assert.match(source, /await getSetupStatus\(\)/);
  assert.match(source, /await getCurrentAccount\(\)/);
  assert.match(source, /gate === 'authenticated' && account/);
  assert.match(source, /<AuthenticatedApp/);
  assert.match(source, /onAuthenticationLoss/);
  assert.match(source, /gate = 'logout-failed'/);
  assert.match(source, /await getRegistrationPolicy\(\)/);
  assert.match(source, /<RegistrationPage/);
});
