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
  it('retains continuation through three committed batches', () => {
    const { controller, streams } = harness(); controller.start();
    streams[0]?.handlers.onStart({ offset: 0, eligible: 130, sourcesInBatch: 50, requestedConcurrency: 7, retryCursor: 'retry-0', effectiveConcurrency: 7 });
    streams[0]?.handlers.onDone({ complete: true, checked: 50, eligible: 130, hasMore: true, nextCursor: 'next-50', retryCursor: 'retry-0', sourceFailures: 0 });
    controller.more(); expect(streams[1]?.options.cursor).toBe('next-50');
    streams[1]?.handlers.onStart({ offset: 50, eligible: 130, sourcesInBatch: 50, requestedConcurrency: 7, retryCursor: 'retry-50', effectiveConcurrency: 7 });
    streams[1]?.handlers.onDone({ complete: true, checked: 100, eligible: 130, hasMore: true, nextCursor: 'next-100', retryCursor: 'retry-50', sourceFailures: 0 });
    expect(controller.state.hasMore).toBe(true); controller.more(); expect(streams[2]?.options.cursor).toBe('next-100');
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
    streams[0]?.handlers.onResult('old', [{ name: 'Old', author: '', coverUrl: '', intro: '', kind: '', lastChapter: '', sourceId: 'old', sourceUrl: 'old', sourceName: 'old', bookUrl: '/old' }], 20);
    expect(controller.state.resultCount).toBe(0); expect(streams[1]?.options.cursor).toBe('retry-10');
  });
});
