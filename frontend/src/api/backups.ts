import { request, requestControl, requestBinary, requestUpload } from './transport';

export interface RestoreResult { restored: boolean; warnings?: string[] }

export interface RestoreStatus extends PreparedRestore {
  state: 'prepared' | 'committing' | 'committed' | 'failed';
  result?: RestoreResult;
}

export interface PreparedRestore {
  operationId: string;
  createdAt: string;
  exportedFromUsername: string;
  readerSchemaVersion: number;
  currentSchemaVersion: number;
  compatibility: 'compatible';
  expiresAt: string;
}

export interface BackupToken {
  id: string;
  name: string;
  canExport: boolean;
  canRestore: boolean;
  createdAt: number;
  expiresAt?: number;
  lastUsedAt?: number;
}

export interface BackupTokenCredential extends BackupToken { token: string }

export async function downloadBackup() {
  const response = await requestBinary('/backups/export');
  const blob = await response.blob();
  const disposition = response.headers.get('Content-Disposition') ?? '';
  const encoded = disposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1];
  const fallback = disposition.match(/filename="([^"]+)"/i)?.[1];
  return { blob, filename: encoded ? decodeURIComponent(encoded) : fallback || 'novelreader-reader-backup.tar.gz' };
}

export function prepareRestore(file: File) { return requestUpload<PreparedRestore>('/backups/restores', file); }
export function getRestoreStatus(operationId: string) { return requestControl<RestoreStatus>(`/backups/restores/${encodeURIComponent(operationId)}`, { signal: AbortSignal.timeout(10000) }); }
export function cancelRestore(operationId: string) { return requestControl<void>(`/backups/restores/${encodeURIComponent(operationId)}`, { method: 'DELETE', signal: AbortSignal.timeout(10000) }); }
export function commitRestore(operationId: string) { return requestControl<RestoreResult>(`/backups/restores/${encodeURIComponent(operationId)}/commit`, { method: 'POST', signal: AbortSignal.timeout(120000) }); }

export async function listBackupTokens() {
  const response = await request<{ tokens: BackupToken[] | null }>('/auth/backup-tokens');
  return response.tokens ?? [];
}
export function createBackupToken(input: { name: string; canExport: boolean; canRestore: boolean; currentPassword?: string; expiresAt?: number }) {
  return request<BackupTokenCredential>('/auth/backup-tokens', { method: 'POST', body: JSON.stringify(input) });
}
export function revokeBackupToken(tokenId: string) { return request<void>(`/auth/backup-tokens/${encodeURIComponent(tokenId)}`, { method: 'DELETE' }); }
