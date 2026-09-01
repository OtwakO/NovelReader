import type { SearchResult } from './models';
import { request } from './transport';
export type { AltSource, SearchResult } from './models';
export interface SearchBatchOptions { batchSize: number; concurrency: number; cursor?: string }
export interface SearchBatchStart { offset: number; eligible: number; sourcesInBatch: number; requestedConcurrency: number; effectiveConcurrency: number; retryCursor: string }
export interface SearchBatchDone { complete: boolean; checked: number; eligible: number; hasMore: boolean; nextCursor?: string; retryCursor: string; sourceFailures: number }
export interface SearchBatchHandlers {
  onStart: (event: SearchBatchStart) => void;
  onResult: (sourceId: string, items: SearchResult[], checked: number) => void;
  onSourceError: (sourceId: string, message: string, checked: number) => void;
  onDone: (event: SearchBatchDone) => void;
  onStale: (message: string) => void;
  onDisconnect: () => void;
}

export function searchInstalledSource(sourceId: string, query: string) {
  return request<SearchResult[]>('/search/source', { method: 'POST', body: JSON.stringify({ sourceId, query }) });
}

export function searchBooksBatchStream(query: string, options: SearchBatchOptions, handlers: SearchBatchHandlers): EventSource {
  const params = new URLSearchParams({ q: query, batchSize: String(options.batchSize), concurrency: String(options.concurrency) });
  if (options.cursor) params.set('cursor', options.cursor);
  const stream = new EventSource(`/api/search/stream?${params}`);
  let finished = false;
  stream.onmessage = (message) => {
    try {
      const event = JSON.parse(message.data) as Record<string, unknown>;
      if (event.type === 'start') handlers.onStart(event as unknown as SearchBatchStart);
      else if (event.type === 'results') handlers.onResult(String(event.sourceId), event.data as SearchResult[], Number(event.checked));
      else if (event.type === 'source_error') handlers.onSourceError(String(event.sourceId), String(event.message), Number(event.checked));
      else if (event.type === 'stale') { finished = true; handlers.onStale(String(event.message)); stream.close(); }
      else if (event.type === 'done') { finished = true; handlers.onDone(event as unknown as SearchBatchDone); stream.close(); }
    } catch { /* malformed stream events cannot safely mutate search state */ }
  };
  stream.onerror = () => {
    if (finished) return;
    finished = true;
    stream.close();
    handlers.onDisconnect();
  };
  return stream;
}
