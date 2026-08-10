import test from 'node:test';
import assert from 'node:assert/strict';
import { clearedPasswordFields, passwordChangeSucceeded } from './accountGate.mjs';

test('successful password change closes private account state and requires login', () => {
  assert.deepEqual(passwordChangeSucceeded(), {
    gate: 'login',
    account: null,
    hash: '#/login',
    message: 'Password changed. Sign in again with your new password.',
  });
});

test('password form clears every credential field after a completed attempt', () => {
  assert.deepEqual(clearedPasswordFields(), {
    currentPassword: '',
    newPassword: '',
    confirmPassword: '',
  });
});
