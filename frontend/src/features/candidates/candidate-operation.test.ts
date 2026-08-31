import { describe, expect, it, vi } from 'vitest';
import {
  cancelCandidateOperation,
  candidateOperationMatches,
  candidateWasCommitted,
  clearCandidateCommittedBook,
  commitCandidateOperation,
  rememberCandidateCommitted,
  startCandidateOperation,
} from './candidate-operation';

const result = {
  name: 'Fixture Novel', author: 'Fixture Author', coverUrl: '', intro: '', kind: '', lastChapter: '',
  bookUrl: '/book', sourceId: 'source', sourceUrl: 'source', sourceName: 'Primary', score: 9,
  alternateSources: [{ sourceId: 'other', sourceUrl: 'other', bookUrl: '/other', sourceName: 'Other' }],
};

describe('candidate operation client', () => {
  it('starts one asynchronous operation and strips client-only fields', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'operation', state: 'running', known: 2, completed: 0, active: 1, attempts: [], updatedAt: new Date().toISOString() }), { status: 202, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    await startCandidateOperation(result, 'book-id');
    const call = fetchMock.mock.calls[0];
    expect(call?.[0]).toContain('/candidate-resolutions');
    const body = JSON.parse(String(call?.[1]?.body));
    expect(body).toMatchObject({ shelveBookId: 'book-id', sourceId: 'source', sourceUrl: 'source', alternateSources: result.alternateSources });
    expect(body.score).toBeUndefined();
  });

  it('compares the complete ordered source binding set', () => {
    const matching = { attempts: [
      { sourceId: 'source', sourceUrl: 'source', bookUrl: '/book' },
      { sourceId: 'other', sourceUrl: 'other', bookUrl: '/other' },
    ] };
    const stale = { attempts: [{ sourceId: 'source', sourceUrl: 'source', bookUrl: '/book' }] };
    expect(candidateOperationMatches(result, matching as never)).toBe(true);
    expect(candidateOperationMatches({ ...result, sourceId: ' source ', sourceUrl: ' source ', bookUrl: ' /book ' }, matching as never)).toBe(true);
    expect(candidateOperationMatches(result, stale as never)).toBe(false);
  });

  it('recognizes committed logical books across different source bindings', () => {
    rememberCandidateCommitted(result, 'stored-book');
    expect(candidateWasCommitted({ ...result, sourceId: 'second-source', sourceUrl: 'second-source', bookUrl: '/second' })).toBe(true);
  });

  it('invalidates committed candidate markers when their stored book is deleted', () => {
    rememberCandidateCommitted(result, 'stored-book');
    expect(candidateWasCommitted(result)).toBe(true);

    clearCandidateCommittedBook('stored-book');

    expect(candidateWasCommitted(result)).toBe(false);
    expect(sessionStorage.getItem('novelreader.candidate-operations.v1')).toBeNull();
  });

  it('uses explicit cancel and idempotent commit endpoints', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: 'operation', state: 'committed', storedBook: { id: 'book-id' } }), { status: 201, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    await cancelCandidateOperation('operation');
    await commitCandidateOperation('operation', 'book-id');
    expect(fetchMock.mock.calls[0]?.[0]).toContain('/candidate-resolutions/operation');
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe('DELETE');
    expect(fetchMock.mock.calls[1]?.[0]).toContain('/candidate-resolutions/operation/shelve');
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toEqual({ bookId: 'book-id' });
  });
});
