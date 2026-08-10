import test from 'node:test';
import assert from 'node:assert/strict';
import { mayManageReaders, readerStatusControl } from './readerAdministration.mjs';

test('only Administrators may see reader management', () => {
  assert.equal(mayManageReaders('admin'), true);
  assert.equal(mayManageReaders('reader'), false);
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
