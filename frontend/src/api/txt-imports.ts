import { request } from './transport';
import { importControl as control } from './file-imports';

export type TXTState = 'receiving' | 'received' | 'analyzing' | 'ready' | 'needs_review' | 'analysis_failed' | 'failed' | 'published' | 'removing';
export type TXTEncoding = '' | 'utf-8' | 'utf-16le' | 'utf-16be' | 'gb18030' | 'big5';
export type TXTPreset = '' | 'chinese-chapters' | 'english-chapters' | 'generated-sections' | 'custom';
export interface TXTOptions { encoding: TXTEncoding; preset: TXTPreset; pattern?: string }
export interface TXTReceipt extends TXTOptions {
  id: string; originalName: string; state: TXTState; size: number; createdAt: number; updatedAt: number;
  libraryId?: string; analysisVersion: number; hasError: boolean; errorCode?: string;
}
export interface TXTPage<T> { items: T[]; nextCursor?: string }
export interface TXTWarnings { warnings?: string[] }
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
export const getTXTReceipt = (id: string, signal: AbortSignal) => control<TXTReceipt>(idPath(id), signal);
export const listTXTReceipts = (after: string, state: string, signal: AbortSignal) => control<TXTPage<TXTReceipt>>(`${base}/receipts?${new URLSearchParams({ after, state, limit: '25' })}`, signal);
export const previewTXT = (id: string, version: number, start: number, signal: AbortSignal) => control<TXTPreview>(`${idPath(id)}/preview?analysisVersion=${version}&start=${start}&limit=25`, signal);
export const analyzeTXT = (id: string, analysisVersion: number, options: TXTOptions, signal: AbortSignal) => control<TXTWarnings>(`${idPath(id)}/analysis`, signal, 'POST', { analysisVersion, ...options });
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

export interface TXTReparseStatus {
  name: string;
  activeGeneration: number; contentRevision: number; stateVersion: number; activeOptions: TXTOptions;
  candidate?: { generation: number; state: 'queued' | 'analyzing' | 'ready' | 'needs_review' | 'analysis_failed'; options: TXTOptions; baseContentRevision: number; hasError: boolean; errorCode?: string };
}
export interface TXTReparseImpact {
  generation: number; activeGeneration: number; contentRevision: number; stateVersion: number; totalSections: number;
  resume: { chapterIndex: number; chapterTitle: string; position: number } | null;
  preservedBookmarks: number; unresolvedBookmarks: number;
}
export interface TXTApplyRequest { generation: number; activeGeneration: number; contentRevision: number; stateVersion: number; resumeChapter?: number }
const reparsePath = (id: string) => `/books/${encodeURIComponent(id)}/txt/reparse`;
export const getTXTReparse = (id: string, signal: AbortSignal) => control<TXTReparseStatus>(reparsePath(id), signal);
export const prepareTXTReparse = (id: string, contentRevision: number, generation: number, options: TXTOptions, signal: AbortSignal) => control<TXTWarnings & { generation: number }>(reparsePath(id), signal, 'POST', { contentRevision, generation, ...options });
export const discardTXTReparse = (id: string, contentRevision: number, generation: number, signal: AbortSignal) => control<void>(reparsePath(id), signal, 'DELETE', { contentRevision, generation });
export const previewTXTReparse = (id: string, generation: number, start: number, signal: AbortSignal) => control<TXTPreview>(`${reparsePath(id)}/preview?generation=${generation}&start=${start}&limit=25`, signal);
export const impactTXTReparse = (id: string, generation: number, signal: AbortSignal) => control<TXTReparseImpact>(`${reparsePath(id)}/impact?generation=${generation}`, signal);
export const applyTXTReparse = (id: string, input: TXTApplyRequest, signal: AbortSignal) => control<{ libraryId: string; contentRevision: number; stateVersion: number; alreadyApplied: boolean }>(`${reparsePath(id)}/apply`, signal, 'POST', input);
