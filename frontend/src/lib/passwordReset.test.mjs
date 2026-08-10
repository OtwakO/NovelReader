import test from 'node:test';
import assert from 'node:assert/strict';
import { clearedResetFields, passwordsMatch, publicResetGate, resetDelivery } from './passwordReset.mjs';

test('password reset fields clear after every completed attempt', () => {
  assert.deepEqual(clearedResetFields(), { token: '', newPassword: '', confirmPassword: '' });
});

test('password reset confirmation must match exactly', () => {
  assert.equal(passwordsMatch('long enough password', 'long enough password'), true);
  assert.equal(passwordsMatch('long enough password', 'different password'), false);
});

test('public password reset route clears any private account before authentication lookup', () => {
  assert.deepEqual(publicResetGate('#/password-reset'), { gate: 'password-reset', account: null });
  assert.equal(publicResetGate('#/shelf'), null);
});

test('issued credential is bound to its reader for transient delivery', () => {
  assert.deepEqual(
    resetDelivery({ id: 'reader-1', username: 'Bob' }, { token: 'secret', expiresAt: 1800 }),
    { readerID: 'reader-1', username: 'Bob', token: 'secret', expiresAt: 1800 },
  );
});
