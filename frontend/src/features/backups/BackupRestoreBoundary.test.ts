import { mount } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { expect, it, vi } from 'vitest';
import BackupRestoreView from './BackupRestoreView.vue';
import { commitRestore, getRestoreStatus, listBackupTokens } from '../../api/backups';
import { useSessionStore } from '../../stores/session';
import { forgetRestore } from './restore-session';
import { NetworkError } from '../../api/transport';
import { useImportQueue } from '../imports/import-queue';

vi.mock('../../api/backups', () => ({ commitRestore: vi.fn(), listBackupTokens: vi.fn(), getRestoreStatus: vi.fn() }));

it('retires old-home uploads before restore and does not offer replay after a lost response', async () => {
  vi.mocked(listBackupTokens).mockResolvedValue([]);
  vi.mocked(getRestoreStatus).mockRejectedValue(new NetworkError());
  const pinia = createPinia();
  useSessionStore(pinia).authenticated({ id: 'alice', username: 'Alice', role: 'reader' });
  const queue = useImportQueue(pinia);
  queue.pause();
  queue.enqueue([new File(['chapter'], 'pending.txt')]);
  let queuedAtCommit = -1;
  vi.mocked(commitRestore).mockImplementation(async () => {
    queuedAtCommit = queue.entries.length;
    // The server has committed; only its response is lost.
    throw new NetworkError();
  });
  const wrapper = mount(BackupRestoreView, {
    global: { plugins: [pinia], mocks: { $t: (key: string) => key, $tm: () => [] } },
  });
  wrapper.vm.prepared = { operationId: 'restore', createdAt: '', exportedFromUsername: 'Alice', readerSchemaVersion: 12, currentSchemaVersion: 12, compatibility: 'compatible', expiresAt: '' };
  wrapper.vm.confirmation = 'backups.restore.confirmWord';
  try {
    await wrapper.vm.commit();
    expect.soft(queuedAtCommit).toBe(0);
    expect.soft(queue.entries).toHaveLength(0);
    expect.soft(wrapper.vm.canCommit).toBe(false);
  } finally {
    wrapper.unmount();
    forgetRestore();
    queue.resetReaderState();
  }
});
