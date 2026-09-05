import { createPinia } from 'pinia';
import { afterEach, expect, it, vi } from 'vitest';
import { installReaderStateBoundary } from './reader-state';
import { useSessionStore } from '../stores/session';
import { useSearchStore } from '../features/search/search-store';
import { useExploreStore } from '../features/explore/explore-store';
import { candidateWasCommitted, rememberCandidateCommitted } from '../features/candidates/candidate-operation';
import { loadCandidateSelection, saveCandidateSelection } from '../features/search/candidate-selection';
import { getProgressVersion, setProgressVersion } from '../features/reader/progress-writer';
import * as auth from '../api/auth';
import * as searchApi from '../api/search';

const alice = { id: 'alice', username: 'Alice', role: 'reader' as const };
const bob = { id: 'bob', username: 'Bob', role: 'reader' as const };
const candidate = { name: 'A Novel', author: 'An Author', sourceId: 'source-a', sourceUrl: 'https://source.test', bookUrl: '/book', sourceName: 'Source', coverUrl: '', intro: '', kind: '', lastChapter: '' };
afterEach(() => { sessionStorage.clear(); localStorage.clear(); vi.restoreAllMocks(); });

it('clears reader-owned state and invalidates late discovery events before another account starts', async () => {
  const pinia = createPinia();
  const stop = installReaderStateBoundary(pinia);
  try {
    const session = useSessionStore(pinia);
    session.authenticated(alice);
    let oldHandlers!: searchApi.SearchBatchHandlers;
    const close = vi.fn();
    vi.spyOn(searchApi, 'searchBooksBatchStream').mockImplementation((_query, _options, handlers) => {
      oldHandlers = handlers;
      return { close } as unknown as EventSource;
    });
    const search = useSearchStore(pinia);
    search.query = 'private query'; search.search();
    const oldResult = oldHandlers.onResult;
    const explore = useExploreStore(pinia);
    explore.sourceId = 'source-a'; explore.save();
    rememberCandidateCommitted(candidate, 'alice-book');
    saveCandidateSelection('selection', candidate);
    setProgressVersion('alice-book', 1);
    localStorage.setItem('novelreader.reader.preferences.v1', 'appearance');
    vi.spyOn(auth, 'logout').mockResolvedValue(undefined);
    await session.logout();
    session.authenticated(bob);
    expect(search.query).toBe('');
    expect(explore.sourceId).toBe('');
    expect(close).toHaveBeenCalled();
    expect(candidateWasCommitted(candidate)).toBe(false);
    expect(loadCandidateSelection('selection')).toBeNull();
    expect(getProgressVersion('alice-book')).toBeUndefined();
    expect(localStorage.getItem('novelreader.reader.preferences.v1')).toBe('appearance');
    search.query = 'new query'; search.search();
    oldResult('source-a', [candidate], 1);
    expect(search.results).toEqual([]);
    search.stop(false);
  } finally { stop(); }
});

it('preserves same-reader tab restoration but resets state on direct account replacement', () => {
  const first = createPinia();
  const stopFirst = installReaderStateBoundary(first);
  useSessionStore(first).authenticated(alice);
  useSearchStore(first).query = 'saved query';
  useSearchStore(first).save();
  stopFirst();
  const reloaded = createPinia();
  const stop = installReaderStateBoundary(reloaded);
  try {
    const session = useSessionStore(reloaded);
    session.authenticated(alice);
    const search = useSearchStore(reloaded);
    search.initialize();
    expect(search.query).toBe('saved query');
    session.authenticated(bob);
    expect(search.query).toBe('');
    expect(sessionStorage.getItem('novelreader.search-session')).toBeNull();
  } finally { stop(); }
});
