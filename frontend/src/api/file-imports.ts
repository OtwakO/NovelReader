import { request } from './transport';

export type ImportFormat = 'txt' | 'epub';
export interface ImportAdmission {
  id: string; state: 'waiting' | 'granted' | 'transferring'; expiresAt: string;
  limits: Record<ImportFormat, number>;
}

export function importControl<T>(path: string, signal: AbortSignal, method = 'GET', body?: object): Promise<T> {
  return request<T>(path, { method, signal: AbortSignal.any([signal, AbortSignal.timeout(30_000)]), body: body === undefined ? undefined : JSON.stringify(body) });
}
export const requestImportAdmission = (signal: AbortSignal) => importControl<ImportAdmission>('/imports/admission', signal, 'POST');
export const getImportAdmission = (id: string, signal: AbortSignal) => importControl<ImportAdmission>(`/imports/admission/${encodeURIComponent(id)}`, signal);
