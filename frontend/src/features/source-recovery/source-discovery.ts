import type { SearchBatchHandlers, SearchBatchOptions, SearchResult } from '../../api/search';

export interface SourceDiscoveryState {
  searching: boolean; checked: number; eligible: number; resultCount: number; effectiveConcurrency: number;
  sourceFailures: number; errorCode: '' | 'missing-query' | 'stale' | 'disconnect'; errorDetail: string;
  restartRequired: boolean; retryRequired: boolean; hasMore: boolean;
}

export interface SourceDiscoveryStream { close(): void }
export type OpenSourceDiscoveryStream = (query: string, options: SearchBatchOptions, handlers: SearchBatchHandlers) => SourceDiscoveryStream;

export function createSourceDiscovery(options: {
  query: () => string;
  preferences: () => { batchSize: number; concurrency: number };
  openStream: OpenSourceDiscoveryStream;
  onResults?: (items: SearchResult[]) => void;
  onChange?: (state: SourceDiscoveryState) => void;
}) {
  let generation = 0;
  let stream: SourceDiscoveryStream | null = null;
  let nextCursor = '';
  let retryCursor = '';
  let committedFailures = 0;
  let batchFailures = 0;
  const state: SourceDiscoveryState = { searching: false, checked: 0, eligible: 0, resultCount: 0, effectiveConcurrency: 0, sourceFailures: 0, errorCode: '', errorDetail: '', restartRequired: false, retryRequired: false, hasMore: false };
  const changed = () => options.onChange?.({ ...state });

  function cancel(markRetry = false) {
    stream?.close(); stream = null; generation += 1; state.searching = false;
    if (markRetry && retryCursor) state.retryRequired = true;
    changed();
  }

  function run(cursor = '', reset = false) {
    cancel(false);
    if (reset) {
      Object.assign(state, { checked: 0, eligible: 0, resultCount: 0, sourceFailures: 0 });
      committedFailures = 0; batchFailures = 0; nextCursor = ''; retryCursor = '';
    }
    const query = options.query().trim();
    if (!query) { state.errorCode = 'missing-query'; changed(); return; }
    const currentGeneration = generation;
    const configuration = options.preferences();
    Object.assign(state, { searching: true, errorCode: '', errorDetail: '', restartRequired: false, retryRequired: false, hasMore: false });
    batchFailures = 0; changed();
    stream = options.openStream(query, { cursor, batchSize: configuration.batchSize, concurrency: configuration.concurrency }, {
      onStart(event) { if (currentGeneration !== generation) return; state.eligible = event.eligible; state.effectiveConcurrency = event.effectiveConcurrency; retryCursor = event.retryCursor || cursor; changed(); },
      onResult(_sourceId, items, checked) { if (currentGeneration !== generation) return; state.checked = checked; state.resultCount += items.length; options.onResults?.(items); changed(); },
      onSourceError(_sourceId, _message, checked) { if (currentGeneration !== generation) return; state.checked = checked; batchFailures += 1; state.sourceFailures = committedFailures + batchFailures; changed(); },
      onDone(event) {
        if (currentGeneration !== generation) return;
        stream = null; state.searching = false; state.checked = event.checked; state.eligible = event.eligible;
        if (event.complete) {
          committedFailures += event.sourceFailures; batchFailures = 0; state.sourceFailures = committedFailures;
          state.hasMore = event.hasMore; nextCursor = event.nextCursor || ''; retryCursor = event.retryCursor || ''; state.retryRequired = false;
        } else {
          state.sourceFailures = committedFailures + batchFailures; state.retryRequired = true; retryCursor = event.retryCursor || retryCursor || cursor;
        }
        changed();
      },
      onStale(message) { if (currentGeneration !== generation) return; stream = null; state.searching = false; state.errorCode = 'stale'; state.errorDetail = message; state.restartRequired = true; changed(); },
      onDisconnect() { if (currentGeneration !== generation) return; stream = null; state.searching = false; state.errorCode = 'disconnect'; state.retryRequired = true; changed(); },
    });
  }

  return {
    state,
    start: () => run('', true),
    restart: () => run('', true),
    more: () => { if (nextCursor) run(nextCursor, false); },
    retry: () => run(retryCursor, false),
    stop: () => cancel(true),
    destroy: () => cancel(false),
  };
}
