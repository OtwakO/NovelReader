import { request } from './transport';
import { importControl } from './file-imports';

export type EPUBImageMode = 'original' | 'optimized';
export interface EPUBReceipt {
  id: string; originalName: string; state: 'receiving' | 'finalizing' | 'acquired' | 'failed' | 'removing';
  size: number; createdAt: number; updatedAt: number; libraryId?: string; generation: number;
  preparationState?: 'queued' | 'preparing' | 'finalizing' | 'ready' | 'failed';
  imageMode: EPUBImageMode; hasError: boolean; errorCode?: string; notices?: string[];
}
export interface EPUBPreview {
  generation: number; title: string; authors: string[]; language?: string; totalSections: number;
  headings: { index: number; title: string; auxiliary: boolean }[]; hasMore: boolean;
  sample: string; sampleTruncated: boolean; needsReview: boolean; diagnostics: string[]; notices?: string[];
  images: { mode: EPUBImageMode; profile?: string; backend?: 'native' | 'portable'; derivativeCount: number; derivativeBytes: number };
}
export interface EPUBAcquisition { receipt: EPUBReceipt; warnings?: string[] }
const base = '/imports/epub';
const receiptPath = (id: string) => `${base}/receipts/${encodeURIComponent(id)}`;
export const getEPUBReceipt = (id: string, signal: AbortSignal) => importControl<EPUBReceipt>(receiptPath(id), signal);
export const listEPUBReceipts = (after: string, signal: AbortSignal) => importControl<{ items: EPUBReceipt[]; nextCursor?: string }>(`${base}/receipts?${new URLSearchParams({ after, limit: '25' })}`, signal);
export const previewEPUB = (id: string, generation: number, start: number, signal: AbortSignal) => importControl<EPUBPreview>(`${receiptPath(id)}/preview?generation=${generation}&start=${start}&limit=25`, signal);
export const retryEPUB = (id: string, generation: number, signal: AbortSignal) => importControl<EPUBAcquisition>(`${receiptPath(id)}/retry`, signal, 'POST', { generation });
export const acceptEPUB = (id: string, generation: number, name: string, author: string, signal: AbortSignal) => importControl<{ libraryId: string }>(`${receiptPath(id)}/accept`, signal, 'POST', { generation, name, author });
export const discardEPUB = (id: string, signal: AbortSignal) => importControl<{ cleanupPending: boolean; warnings?: string[] }>(receiptPath(id), signal, 'DELETE');

// Pass the File directly. The server owns parsing, image work and durable output.
export function acquireEPUB(id: string, input: File | string, imageMode: EPUBImageMode, signal: AbortSignal): Promise<EPUBAcquisition> {
  const inbox = typeof input === 'string';
  const name = inbox ? input : input.name;
  const path = inbox ? 'inbox/acquisitions' : 'uploads';
  return request<EPUBAcquisition>(`${base}/${path}/${encodeURIComponent(id)}?${new URLSearchParams({ filename: name, imageMode })}`, {
    method: inbox ? 'POST' : 'PUT', headers: { 'Content-Type': 'application/octet-stream' }, body: inbox ? undefined : input,
    signal: AbortSignal.any([signal, AbortSignal.timeout(31 * 60_000)]),
  });
}
