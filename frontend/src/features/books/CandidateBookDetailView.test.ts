import { flushPromises, mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { saveCandidateSelection } from '../search/candidate-selection';
import CandidateBookDetailView from './CandidateBookDetailView.vue';

const candidateMessages = {
  title: 'Finding source', help: 'First readable source wins', starting: 'Starting', finishing: 'Finishing',
  runningSummary: '{active} active · {completed}/{known}', failedSummary: 'Failed · {completed}/{known}',
  counts: '{completed}/{known} · {active} active', winnerCounts: 'Winner · {active} draining', cancelling: 'Cancelling', cancelFailed: 'Cancel failed',
  queued: '{count} waiting', skipped: '{count} skipped', moreFailed: '{count} more failed',
  stage: { book_info: 'Info', toc: 'TOC', content: 'Content' },
  state: { running: 'Checking', failed: 'Unavailable', verified: 'Readable', skipped: 'Not needed' },
};
const i18n = createI18n({
  legacy: false, globalInjection: true, locale: 'en',
  messages: { en: {
    candidate: candidateMessages,
    candidateBookDetail: {
      title: 'Book details', description: 'Description', loading: 'Loading', disconnected: 'Disconnected', failed: 'Failed', missing: 'Missing', retry: 'Retry', cancel: 'Cancel',
      source: 'Source: {source}', sourceCount: '{count} known', fallback: 'Using {source}', noIntro: 'No intro',
      shelve: 'Add', shelving: 'Adding', shelfFailed: 'Shelf failed', back: 'Back', backExplore: 'Back explore',
    },
    bookDetail: { coverAlt: 'Cover of {name}', synopsis: 'Synopsis', chapters: 'Contents', showAll: 'Show all {count} entries', noChapters: 'No chapters' },
    reader: { toc: { readableSummary: '{readable} readable chapters', summary: '{readable} readable · {total} total', search: 'Search contents', searchPlaceholder: 'Chapter title or entry number', clearSearch: 'Clear', ascending: 'Oldest first', descending: 'Newest first', jumpCurrent: 'Current', matches: '{count} matches', noMatches: 'No matches' } },
    app: { common: { unknownAuthor: 'Unknown' } },
  } },
});

const selected = {
  name: 'Fixture Novel', author: 'Fixture Author', coverUrl: '', intro: 'Intro', kind: 'Novel', lastChapter: 'Chapter 2',
  bookUrl: '/book', sourceUrl: 'primary', sourceName: 'Primary',
  alternateSources: [{ sourceUrl: 'fallback', bookUrl: '/fallback', sourceName: 'Fallback' }],
};

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) { FakeEventSource.instances.push(this); }
  close() {}
  emit(value: unknown) { this.onmessage?.({ data: JSON.stringify(value) } as MessageEvent); }
}

function operation(state = 'running') {
  return { id: 'operation', state, known: 2, completed: 0, active: 1, attempts: [], updatedAt: new Date().toISOString() };
}
function verifiedOperation() {
  return {
    ...operation('verified'), completed: 1, active: 0,
    attempts: [
      { sourceName: 'Primary', sourceUrl: 'primary', bookUrl: '/book', state: 'failed', stage: 'content' },
      { sourceName: 'Fallback', sourceUrl: 'fallback', bookUrl: '/fallback', state: 'verified', stage: 'content' },
    ],
    preview: {
        book: { ...selected, lastChapter: '/raw/path.html', sourceUrl: 'fallback', bookUrl: '/fallback', origin: 'Fallback' },
      chapters: [{ index: 0, title: 'Chapter 1', url: '/chapter' }],
      selection: { requestedSourceUrl: 'primary', selectedSourceUrl: 'fallback', selectedSourceName: 'Fallback', usedFallback: true },
    },
  };
}

beforeEach(() => {
  sessionStorage.clear();
  saveCandidateSelection('candidate-key', selected);
  FakeEventSource.instances = [];
  vi.stubGlobal('EventSource', FakeEventSource);
  vi.stubGlobal('crypto', { randomUUID: () => 'stored-id' });
});

function mountView(replace = vi.fn()) {
  return { wrapper: mount(CandidateBookDetailView, {
    global: {
      plugins: [i18n],
      mocks: { $route: { query: { candidate: 'candidate-key' } }, $router: { replace } },
      stubs: { RouterLink: true, FeatureScaffold: { template: '<main><slot /></main>' }, WebViewFailureHint: true },
    },
  }), replace };
}

describe('CandidateBookDetailView', () => {
  it('renders verified details through shared section and progress components without persisting on open', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(operation()), { status: 202, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    const { wrapper } = mountView();
    await flushPromises();
    FakeEventSource.instances[0]?.emit(verifiedOperation());
    await wrapper.vm.$nextTick();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(wrapper.text()).toContain('Synopsis');
    expect(wrapper.text()).not.toContain('bookDetail.introduction');
    expect(wrapper.find('.book-detail-section__body').exists()).toBe(true);
    expect(wrapper.text()).toContain('Contents');
    expect(wrapper.text()).toContain('Chapter 1');
    expect(wrapper.text()).toContain('Using Fallback');
    expect(wrapper.text()).not.toContain('/raw/path.html');
    expect(wrapper.text()).not.toContain('Current');
  });

  it('keeps following an automatic operation from verified through committed', async () => {
    const automatic = { ...operation(), automaticCommit: true };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(automatic), { status: 202, headers: { 'Content-Type': 'application/json' } })));
    const replace = vi.fn();
    const { wrapper } = mountView(replace);
    await flushPromises();

    FakeEventSource.instances[0]?.emit({ ...verifiedOperation(), automaticCommit: true });
    await wrapper.vm.$nextTick();
    expect(wrapper.text()).toContain('Adding');

    FakeEventSource.instances[0]?.emit({ ...verifiedOperation(), automaticCommit: true, state: 'committed', storedBook: { id: 'stored-id' } });
    await flushPromises();
    expect(replace).toHaveBeenCalledWith('/books/stored-id');
  });

  it('restarts when Candidate Detail restores an operation with an older source set', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['primary\u0000/book']: 'operation' }));
    const stale = {
      ...verifiedOperation(), known: 1,
      attempts: [{ sourceName: 'Primary', sourceUrl: 'primary', bookUrl: '/book', state: 'verified', stage: 'content' }],
    };
    const fresh = operation();
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(stale), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(fresh), { status: 202, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);

    mountView();
    await flushPromises();

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(String(fetchMock.mock.calls[1]?.[0])).toContain('/candidate-resolutions');
    expect((fetchMock.mock.calls[1]?.[1] as RequestInit)?.method).toBe('POST');
  });

  it('redirects a restored committed operation to stored Book Detail', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['primary\u0000/book']: 'operation' }));
    const snapshot = { ...verifiedOperation(), state: 'committed', storedBook: { id: 'stored-id' } };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(snapshot), { status: 200, headers: { 'Content-Type': 'application/json' } })));
    const replace = vi.fn();
    mountView(replace);
    await flushPromises();
    expect(replace).toHaveBeenCalledWith('/books/stored-id');
  });

  it('retries a pending commit from verified data without starting a new crawl', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['primary\u0000/book']: 'operation' }));
    const pending = { ...verifiedOperation(), state: 'failed', commitPending: true, automaticCommit: true, message: 'Storage unavailable' };
    const committed = { ...pending, state: 'committed', commitPending: false, storedBook: { id: 'stored-id' } };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(pending), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(committed), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    const replace = vi.fn();
    const { wrapper } = mountView(replace);
    await flushPromises();

    expect(wrapper.text()).toContain('Synopsis');
    await wrapper.get('.hero-actions button').trigger('click');
    await flushPromises();

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(String(fetchMock.mock.calls[1]?.[0])).toContain('/candidate-resolutions/operation/shelve');
    expect(JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit)?.body))).toEqual({ bookId: '' });
    expect(replace).toHaveBeenCalledWith('/books/stored-id');
  });

  it('keeps verified detail visible while an automatic operation reconnects', async () => {
    const automatic = { ...operation(), automaticCommit: true };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(automatic), { status: 202, headers: { 'Content-Type': 'application/json' } })));
    const { wrapper } = mountView();
    await flushPromises();

    FakeEventSource.instances[0]?.emit({ ...verifiedOperation(), automaticCommit: true });
    FakeEventSource.instances[0]?.onerror?.();
    await wrapper.vm.$nextTick();

    expect(wrapper.text()).toContain('Synopsis');
    expect(wrapper.text()).toContain('Disconnected');
    expect(wrapper.find('.hero').exists()).toBe(true);
  });

  it('keeps a commit failure visible on the verified detail', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(verifiedOperation()), { status: 202, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ message: 'Storage unavailable' }), { status: 500, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    const { wrapper } = mountView();
    await flushPromises();
    await wrapper.get('.hero-actions button').trigger('click');
    await flushPromises();
    expect(wrapper.text()).toContain('Storage unavailable');
    expect(wrapper.text()).toContain('Synopsis');
  });
});
