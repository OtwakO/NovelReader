import { request } from './transport';

export type TXTState = 'receiving' | 'received' | 'analyzing' | 'ready' | 'needs_review' | 'analysis_failed' | 'failed' | 'published' | 'removing';
export type TXTEncoding = '' | 'utf-8' | 'utf-16le' | 'utf-16be' | 'gb18030' | 'big5';
export type TXTPreset = '' | 'chinese-chapters' | 'english-chapters' | 'generated-sections';
export interface TXTReceipt {
  id: string; originalName: string; state: TXTState; size: number; createdAt: number; updatedAt: number;
  libraryId?: string; analysisVersion: number; encoding: TXTEncoding; preset: TXTPreset; hasError: boolean;
}
export interface TXTPage<T> { items: T[]; nextCursor?: string }
export interface TXTWarnings { warnings?: string[] }
export interface TXTAdmission { id: string; state: 'waiting' | 'granted' | 'transferring'; expiresAt: string; maxInputBytes: number }
export interface TXTAcquisition extends TXTWarnings { receipt: TXTReceipt }
export interface TXTPreview {
  analysisVersion: number; encoding: TXTEncoding; preset: TXTPreset; parserVersion: number;
  reviewReasons: string[]; totalSections: number;
  headings: { index: number; title: string; generated: boolean }[];
  hasMore: boolean; sample: string; sampleTruncated: boolean;
}
export interface TXTInboxClaim { name: string; receiptId: string }
export interface TXTInboxEntry { name: string; size: number; modifiedAt: number; receiptId?: string; problem?: string }
export interface TXTInboxReview extends TXTWarnings {
  token: string; expiresAt: string; name: string; receiptId: string; inputPresent: boolean; canRemove: boolean;
}
const base = '/imports/txt';
const idPath = (id: string) => `${base}/receipts/${encodeURIComponent(id)}`;
function control<T>(path: string, signal: AbortSignal, method = 'GET', body?: object): Promise<T> {
  return request<T>(path, { method, signal: AbortSignal.any([signal, AbortSignal.timeout(30_000)]), body: body === undefined ? undefined : JSON.stringify(body) });
}
export const requestTXTAdmission = (signal: AbortSignal) => control<TXTAdmission>(`${base}/admission`, signal, 'POST');
export const getTXTAdmission = (id: string, signal: AbortSignal) => control<TXTAdmission>(`${base}/admission/${encodeURIComponent(id)}`, signal);
export const getTXTReceipt = (id: string, signal: AbortSignal) => control<TXTReceipt>(idPath(id), signal);
export const listTXTReceipts = (after: string, state: string, signal: AbortSignal) => control<TXTPage<TXTReceipt>>(`${base}/receipts?${new URLSearchParams({ after, state, limit: '25' })}`, signal);
export const previewTXT = (id: string, version: number, start: number, signal: AbortSignal) => control<TXTPreview>(`${idPath(id)}/preview?analysisVersion=${version}&start=${start}&limit=25`, signal);
export const analyzeTXT = (id: string, analysisVersion: number, encoding: TXTEncoding, preset: TXTPreset, signal: AbortSignal) => control<TXTWarnings>(`${idPath(id)}/analysis`, signal, 'POST', { analysisVersion, encoding, preset });
export const acceptTXT = (id: string, analysisVersion: number, name: string, author: string, signal: AbortSignal) => control<{ libraryId: string }>(`${idPath(id)}/accept`, signal, 'POST', { analysisVersion, name, author });
export const discardTXT = (id: string, signal: AbortSignal) => control<TXTWarnings & { status: string }>(idPath(id), signal, 'DELETE');
export const scanTXTInbox = (after: string, signal: AbortSignal) => control<TXTPage<TXTInboxEntry> & { directory: string }>(`${base}/inbox?${new URLSearchParams({ after, limit: '25' })}`, signal);
export const listTXTInboxClaims = (after: string, signal: AbortSignal) => control<TXTPage<TXTInboxClaim>>(`${base}/inbox/claims?${new URLSearchParams({ after, limit: '25' })}`, signal);
export const reviewTXTInbox = (id: string, signal: AbortSignal) => control<TXTInboxReview>(`${base}/inbox/claims/${encodeURIComponent(id)}/review`, signal, 'POST');
export const resolveTXTInbox = (token: string, action: 'confirm' | 'release', signal: AbortSignal) => control<void>(`${base}/inbox/reviews/${encodeURIComponent(token)}/${action}`, signal, 'POST');
export const cancelTXTInboxReview = (token: string, signal: AbortSignal) => control<void>(`${base}/inbox/reviews/${encodeURIComponent(token)}`, signal, 'DELETE');

// File is passed directly to fetch: no FileReader, decoding, or intermediate copy.
export function acquireTXT(id: string, input: File | string, signal: AbortSignal): Promise<TXTAcquisition> {
  const inbox = typeof input === 'string';
  const name = inbox ? input : input.name;
  const path = inbox ? 'inbox/acquisitions' : 'uploads';
  return request<TXTAcquisition>(`${base}/${path}/${encodeURIComponent(id)}?${new URLSearchParams({ filename: name })}`, {
    method: inbox ? 'POST' : 'PUT', headers: { 'Content-Type': 'application/octet-stream' },
    body: inbox ? undefined : input, signal: AbortSignal.any([signal, AbortSignal.timeout(31 * 60_000)]),
  });
}
