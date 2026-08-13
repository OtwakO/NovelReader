import type { SearchResult } from './models';
import { request } from './transport';
export type { AltSource, SearchResult } from './models';
export function searchBooks(query: string) { return request<SearchResult[]>(`/search?q=${encodeURIComponent(query)}`); }

export function searchBooksStream(
  query: string,
  onResult: (source: string, items: SearchResult[]) => void,
  onError: (source: string, message: string) => void,
  onDone: (total: number, sourcesDone: number) => void,
  onMerged?: (items: SearchResult[]) => void,
): EventSource {
  const stream = new EventSource(`/api/search/stream?q=${encodeURIComponent(query)}`);
  let finished = false;
  stream.onmessage = (message) => {
    try {
      const event = JSON.parse(message.data) as Record<string, unknown>;
      if (event.type === 'results') onResult(String(event.source), event.data as SearchResult[]);
      else if (event.type === 'error') onError(String(event.source), String(event.message));
      else if (event.type === 'done') {
        finished = true;
        onDone(Number(event.total), Number(event.sourcesDone));
        if (Array.isArray(event.merged)) onMerged?.(event.merged as SearchResult[]);
        stream.close();
      }
    } catch { /* malformed stream events cannot safely mutate search state */ }
  };
  stream.onerror = () => {
    if (finished) return;
    finished = true;
    stream.close();
    onDone(0, 0);
    onMerged?.([]);
  };
  return stream;
}

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
