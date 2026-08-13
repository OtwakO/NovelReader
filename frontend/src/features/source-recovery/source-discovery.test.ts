import { describe, expect, it } from 'vitest';
import type { SearchBatchHandlers } from '../../api/search';
import { createSourceDiscovery } from './source-discovery';

function harness() {
  const streams: Array<{ options: { cursor?: string; batchSize: number; concurrency: number }; handlers: SearchBatchHandlers; closed: boolean; close: () => void }> = [];
  const controller = createSourceDiscovery({ query: () => 'Fixture', preferences: () => ({ batchSize: 25, concurrency: 7 }), openStream: (_query, options, handlers) => { const stream = { options, handlers, closed: false, close() { this.closed = true; } }; streams.push(stream); return stream; } });
  return { controller, streams };
}

describe('source discovery', () => {
  it('tracks progress and continues at the next cursor', () => {
    const { controller, streams } = harness(); controller.start();
    streams[0]?.handlers.onStart({ offset: 0, eligible: 80, sourcesInBatch: 25, requestedConcurrency: 7, retryCursor: 'retry-0', effectiveConcurrency: 7 });
    streams[0]?.handlers.onResult('a', [], 1); streams[0]?.handlers.onDone({ complete: true, checked: 25, eligible: 80, hasMore: true, nextCursor: 'next-25', retryCursor: '', sourceFailures: 2 });
    expect(controller.state.checked).toBe(25); expect(controller.state.sourceFailures).toBe(2); controller.more(); expect(streams[1]?.options.cursor).toBe('next-25');
  });
  it('requires exact retry when a batch ends without committing', () => {
    const { controller, streams } = harness(); controller.start();
    streams[0]?.handlers.onStart({ offset: 25, eligible: 80, sourcesInBatch: 25, requestedConcurrency: 7, retryCursor: 'retry-25', effectiveConcurrency: 7 });
    streams[0]?.handlers.onDone({ complete: false, checked: 30, eligible: 80, hasMore: true, nextCursor: 'next-50', retryCursor: 'retry-25', sourceFailures: 1 });
    expect(controller.state.retryRequired).toBe(true); expect(controller.state.hasMore).toBe(false); controller.retry(); expect(streams[1]?.options.cursor).toBe('retry-25');
  });

  it('preserves exact retry and ignores obsolete events', () => {
    const { controller, streams } = harness(); controller.start();
    streams[0]?.handlers.onStart({ offset: 10, eligible: 80, sourcesInBatch: 25, requestedConcurrency: 7, retryCursor: 'retry-10', effectiveConcurrency: 6 });
    controller.stop(); expect(streams[0]?.closed).toBe(true); expect(controller.state.retryRequired).toBe(true); controller.retry();
    streams[0]?.handlers.onResult('old', [{ name: 'Old', author: '', coverUrl: '', intro: '', kind: '', lastChapter: '', sourceUrl: 'old', sourceName: 'old', bookUrl: '/old' }], 20);
    expect(controller.state.resultCount).toBe(0); expect(streams[1]?.options.cursor).toBe('retry-10');
  });
});
