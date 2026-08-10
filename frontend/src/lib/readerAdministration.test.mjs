import test from 'node:test';
import assert from 'node:assert/strict';
import { deletionConfirmationMatches, deletionControl, mayManageReaders, readerStatusControl } from './readerAdministration.mjs';

test('only Administrators may see reader management', () => {
  assert.equal(mayManageReaders('admin'), true);
  assert.equal(mayManageReaders('reader'), false);
});

test('deletion requires exact username once and becomes retry-only while deleting', () => {
  assert.equal(deletionConfirmationMatches('Bob', 'Bob'), true);
  assert.equal(deletionConfirmationMatches('Bob', 'bob'), false);
  assert.deepEqual(deletionControl('active'), { label: 'Delete account', requiresConfirmation: true });
  assert.deepEqual(deletionControl('deleting'), { label: 'Retry deletion', requiresConfirmation: false });
});

test('reader status maps to bounded account actions', () => {
  assert.deepEqual(readerStatusControl('active'), {
    label: 'Disable', enabled: false, confirmDisable: true, available: true,
  });
  assert.deepEqual(readerStatusControl('disabled'), {
    label: 'Re-enable', enabled: true, confirmDisable: false, available: true,
  });
  assert.deepEqual(readerStatusControl('deleting'), {
    label: 'Unavailable', enabled: false, confirmDisable: false, available: false,
  });
});
