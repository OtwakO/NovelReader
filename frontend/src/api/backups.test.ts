import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createBackupToken, downloadBackup, prepareRestore } from './backups';

const request = vi.fn();
const requestBinary = vi.fn();
const requestUpload = vi.fn();
vi.mock('./transport', () => ({
  request: (...args: unknown[]) => request(...args),
  requestBinary: (...args: unknown[]) => requestBinary(...args),
  requestUpload: (...args: unknown[]) => requestUpload(...args),
}));

describe('backup transport', () => {
  beforeEach(() => { request.mockReset(); requestBinary.mockReset(); requestUpload.mockReset(); });

  it('extracts the UTF-8 export filename and uploads the archive directly', async () => {
    requestBinary.mockResolvedValue(new Response(new Blob(['archive']), { headers: { 'Content-Disposition': "attachment; filename=\"backup.tar.gz\"; filename*=UTF-8''novelreader-%E6%B8%AC%E8%A9%A6.tar.gz" } }));
    await expect(downloadBackup()).resolves.toMatchObject({ filename: 'novelreader-測試.tar.gz' });
    const file = new File(['archive'], 'backup.tar.gz', { type: 'application/gzip' });
    requestUpload.mockResolvedValue({ operationId: 'restore-1' });
    await prepareRestore(file);
    expect(requestUpload).toHaveBeenCalledWith('/backups/restores', file);
  });

  it('sends restore authority, password, and expiry only through token creation', async () => {
    request.mockResolvedValue({ token: 'secret' });
    await createBackupToken({ name: 'Nightly', canExport: true, canRestore: true, currentPassword: 'password', expiresAt: 123 });
    expect(request).toHaveBeenCalledWith('/auth/backup-tokens', { method: 'POST', body: JSON.stringify({ name: 'Nightly', canExport: true, canRestore: true, currentPassword: 'password', expiresAt: 123 }) });
  });
});
