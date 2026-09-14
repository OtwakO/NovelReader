import { mount, flushPromises } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { afterEach, expect, it, vi } from 'vitest';
import BackupRestoreView from './BackupRestoreView.vue';
import { cancelRestore, commitRestore, getRestoreStatus, listBackupTokens } from '../../api/backups';
import { ApiError } from '../../api/transport';
import { useSessionStore } from '../../stores/session';
import { forgetRestore } from './restore-session';
import { resetReaderState } from '../../app/reader-state';

vi.mock('../../api/backups', () => ({ commitRestore: vi.fn(), listBackupTokens: vi.fn(), getRestoreStatus: vi.fn(), cancelRestore: vi.fn() }));
vi.mock('../../app/reader-state', () => ({ resetReaderState: vi.fn() }));
afterEach(() => { forgetRestore(); vi.clearAllMocks(); });

function mountReadyRestore() {
  vi.mocked(listBackupTokens).mockResolvedValue([]);
  const pinia = createPinia();
  useSessionStore(pinia).authenticated({ id: 'alice', username: 'Alice', role: 'reader' });
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

it('retires old state and consumes the preparation even when replacement fails', async () => {
  vi.mocked(commitRestore).mockRejectedValue(new Error('Reader is busy'));
  vi.mocked(getRestoreStatus).mockResolvedValue({ operationId: 'restore', state: 'failed' } as Awaited<ReturnType<typeof getRestoreStatus>>);
  const { wrapper } = mountReadyRestore();
  try {
    await wrapper.vm.commit();
    await flushPromises();
    expect(resetReaderState).toHaveBeenCalled();
    expect(wrapper.vm.prepared).toBeNull();
    expect(wrapper.vm.canCommit).toBe(false);
    expect(wrapper.get('[role="alert"]').text()).toBe('backups.restore.commitFailed');
    expect(wrapper.find('[role="status"]').exists()).toBe(false);
  } finally { wrapper.unmount(); }
});

it('recovers a lost response through status without resending replacement', async () => {
  vi.mocked(commitRestore).mockRejectedValue(new Error('Connection lost'));
  vi.mocked(getRestoreStatus)
    .mockResolvedValueOnce({ operationId: 'restore', state: 'committing' } as Awaited<ReturnType<typeof getRestoreStatus>>)
    .mockResolvedValueOnce({ operationId: 'restore', state: 'committed', result: { restored: true, warnings: ['txt_recovery_incomplete'] } } as Awaited<ReturnType<typeof getRestoreStatus>>);
  const { wrapper } = mountReadyRestore();
  try {
    await wrapper.vm.commit();
    expect(wrapper.vm.pendingOperation).toBe('restore');
    expect(wrapper.vm.canCommit).toBe(false);
    await wrapper.vm.commit();
    await wrapper.vm.checkOutcome();
    expect(commitRestore).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.pendingOperation).toBe('');
    expect(wrapper.vm.restoreWarning).toBe('backups.restore.recoveryWarning');
  } finally { wrapper.unmount(); }
});

it('does not treat prepared status as proof an uncertain commit can no longer arrive', async () => {
  vi.mocked(commitRestore).mockRejectedValue(new Error('Connection lost'));
  vi.mocked(getRestoreStatus).mockResolvedValue({ operationId: 'restore', state: 'prepared' } as Awaited<ReturnType<typeof getRestoreStatus>>);
  vi.mocked(cancelRestore).mockResolvedValue(undefined);
  const { wrapper } = mountReadyRestore();
  try {
    await wrapper.vm.commit();
    expect(wrapper.vm.pendingOperation).toBe('restore');
    expect(wrapper.vm.canCommit).toBe(false);
    await wrapper.vm.cancelUnstartedRestore();
    expect(cancelRestore).toHaveBeenCalledWith('restore');
    expect(wrapper.vm.pendingOperation).toBe('');
    expect(wrapper.vm.prepared).toBeNull();
    expect(commitRestore).toHaveBeenCalledTimes(1);
  } finally { wrapper.unmount(); }
});

it('keeps disconnected recovery blocked and requires acknowledgement when the record is gone', async () => {
  vi.mocked(commitRestore).mockRejectedValue(new Error('Connection lost'));
  vi.mocked(getRestoreStatus).mockRejectedValueOnce(new Error('Offline'))
    .mockRejectedValueOnce(new ApiError(404, { code: 'restore_not_found' }));
  const { wrapper } = mountReadyRestore();
  try {
    await wrapper.vm.commit();
    wrapper.vm.acknowledgeUnknown();
    expect(wrapper.vm.pendingOperation).toBe('restore');
    await wrapper.vm.checkOutcome();
    expect(wrapper.vm.outcome).toBe('unknown');
    expect(wrapper.vm.canCommit).toBe(false);
    wrapper.vm.acknowledgeUnknown();
    expect(wrapper.vm.pendingOperation).toBe('');
    expect(wrapper.vm.restoreWarning).toBe('backups.restore.unknown');
    expect(commitRestore).toHaveBeenCalledTimes(1);
  } finally { wrapper.unmount(); }
});
