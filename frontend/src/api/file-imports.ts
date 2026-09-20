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

export interface InboxClaim { name: string; receiptId: string }
export interface InboxEntry { name: string; size: number; modifiedAt: number; receiptId?: string; problem?: string }
export interface InboxReview {
  token: string; expiresAt: string; name: string; receiptId: string; inputPresent: boolean; canRemove: boolean; warnings?: string[];
}
interface InboxPage<T> { items: T[]; nextCursor?: string }
const inboxPath = (format: ImportFormat) => `/imports/${format}/inbox`;
export const scanInbox = (format: ImportFormat, after: string, signal: AbortSignal) => importControl<InboxPage<InboxEntry> & { directory: string }>(`${inboxPath(format)}?${new URLSearchParams({ after, limit: '25' })}`, signal);
export const listInboxClaims = (format: ImportFormat, after: string, signal: AbortSignal) => importControl<InboxPage<InboxClaim>>(`${inboxPath(format)}/claims?${new URLSearchParams({ after, limit: '25' })}`, signal);
export const reviewInbox = (format: ImportFormat, id: string, signal: AbortSignal) => importControl<InboxReview>(`${inboxPath(format)}/claims/${encodeURIComponent(id)}/review`, signal, 'POST');
export const resolveInbox = (format: ImportFormat, token: string, action: 'confirm' | 'release', signal: AbortSignal) => importControl<void>(`${inboxPath(format)}/reviews/${encodeURIComponent(token)}/${action}`, signal, 'POST');
export const cancelInboxReview = (format: ImportFormat, token: string, signal: AbortSignal) => importControl<void>(`${inboxPath(format)}/reviews/${encodeURIComponent(token)}`, signal, 'DELETE');
