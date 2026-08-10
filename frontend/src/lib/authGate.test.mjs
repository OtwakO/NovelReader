import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { compile } from 'svelte/compiler';

const root = path.resolve(import.meta.dirname, '..');

for (const file of ['App.svelte', 'lib/LoginPage.svelte', 'lib/RegistrationPage.svelte', 'lib/PasswordResetPage.svelte', 'lib/AccountPage.svelte', 'lib/AuthenticatedApp.svelte']) {
  test(`${file} compiles`, () => {
    const source = fs.readFileSync(path.join(root, file), 'utf8');
    const result = compile(source, { filename: file, generate: 'client' });
    assert.ok(result.js.code.length > 0);
  });
}

test('administrator account page exposes bounded reader status controls', () => {
  const source = fs.readFileSync(path.join(root, 'lib/AccountPage.svelte'), 'utf8');
  assert.match(source, /mayManageReaders\(account\.role\)/);
  assert.match(source, /await listReaderAccounts\(\)/);
  assert.match(source, /window\.confirm/);
  assert.match(source, /await setReaderEnabled\(reader\.id, enabled\)/);
  assert.match(source, /await issueReaderPasswordReset\(reader\.id\)/);
  assert.match(source, /will not be shown again/);
  assert.match(source, /await deleteReaderAccount\(reader\.id, confirmation\)/);
  assert.match(source, /Permanently delete/);
});

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
  assert.match(source, /onPasswordChanged/);
  assert.match(source, /gate === 'password-reset'/);
  assert.match(source, /<PasswordResetPage/);
});
