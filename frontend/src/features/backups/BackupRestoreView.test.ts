import { mount, flushPromises } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { afterEach, expect, it, vi } from 'vitest';
import BackupRestoreView from './BackupRestoreView.vue';
import { commitRestore, listBackupTokens } from '../../api/backups';
import { resetReaderState } from '../../app/reader-state';

vi.mock('../../api/backups', () => ({ commitRestore: vi.fn(), listBackupTokens: vi.fn() }));
vi.mock('../../app/reader-state', () => ({ resetReaderState: vi.fn() }));
afterEach(() => vi.clearAllMocks());

function mountReadyRestore() {
  vi.mocked(listBackupTokens).mockResolvedValue([]);
  const pinia = createPinia();
  const wrapper = mount(BackupRestoreView, {
    global: { plugins: [pinia], mocks: { $t: (key: string) => key, $tm: () => [] } },
  });
  wrapper.vm.prepared = { operationId: 'restore', createdAt: '', exportedFromUsername: 'Alice', readerSchemaVersion: 11, currentSchemaVersion: 11, compatibility: 'compatible', expiresAt: '' };
  wrapper.vm.confirmation = 'backups.restore.confirmWord';
  return { wrapper, pinia };
}

it('keeps a successful restore warning visible and discards pre-restore reader state', async () => {
  vi.mocked(commitRestore).mockResolvedValue({ restored: true, warnings: ['txt_recovery_incomplete'] });
  const { wrapper, pinia } = mountReadyRestore();
  try {
    await wrapper.vm.commit();
    await flushPromises();
    expect(resetReaderState).toHaveBeenCalledWith(pinia);
    expect(wrapper.get('[role="status"]').text()).toBe('backups.restore.recoveryWarning');
    expect(wrapper.find('[role="alert"]').exists()).toBe(false);
    expect(wrapper.vm.prepared).toBeNull();
    expect(wrapper.vm.committing).toBe(false);
    await wrapper.vm.commit();
    expect(commitRestore).toHaveBeenCalledTimes(1);
  } finally { wrapper.unmount(); }
});

it('preserves the current reader state and preparation when commit is rejected', async () => {
  vi.mocked(commitRestore).mockRejectedValue(new Error('Reader is busy'));
  const { wrapper } = mountReadyRestore();
  try {
    await wrapper.vm.commit();
    await flushPromises();
    expect(resetReaderState).not.toHaveBeenCalled();
    expect(wrapper.vm.prepared?.operationId).toBe('restore');
    expect(wrapper.get('[role="alert"]').text()).toBe('Reader is busy');
    expect(wrapper.find('[role="status"]').exists()).toBe(false);
  } finally { wrapper.unmount(); }
});
