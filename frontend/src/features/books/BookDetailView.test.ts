import { flushPromises, mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { afterEach, describe, expect, it, vi } from 'vitest';
import BookDetailView from './BookDetailView.vue';

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: 'en',
  messages: { en: {
    app: { common: { unknownAuthor: 'Unknown' } },
    bookDetail: {
      title: 'Book details', description: 'Description', loading: 'Loading', loadFailed: 'Load failed', tocFailed: 'TOC failed', tocSyncing: 'Synchronizing the chapter list…', retryToc: 'Retry chapter list', notFound: 'Not found', back: 'Back', coverAlt: 'Cover of {name}', tocEntries: '{count} entries', progress: '{percent}% read', latest: 'Latest: {chapter}', currentSource: 'Current source: {source}', continue: 'Continue', remove: 'Remove', confirmRemoveTitle: 'Remove?', confirmRemoveDescription: 'Remove {name}?', cancel: 'Cancel', confirmRemove: 'Remove', synopsis: 'Synopsis', chapters: 'Chapters', noChapters: 'No chapters', showAll: 'Show all {count}',
    },
    reader: { toc: { readableSummary: '{readable} readable', summary: '{readable}/{total}', search: 'Search', searchPlaceholder: 'Search', clearSearch: 'Clear', ascending: 'Ascending', descending: 'Descending', jumpCurrent: 'Current', matches: '{count} matches', noMatches: 'No matches' } },
    sourceRecovery: { title: 'Sources', cleared: 'Cleared' },
  } },
});

const book = {
  id: 'book-1', name: 'Fixture Novel', author: 'Author', coverUrl: '', intro: '', kind: '', sourceId: 'source-1', sourceUrl: 'source-1', bookUrl: '/book', origin: 'Source', lastChapter: '', durChapterIndex: 0, durChapterPos: 0, totalChapterNum: 0, stateVersion: 0, alternateSources: [],
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('BookDetailView catalog synchronization', () => {
  it('shows book metadata while the chapter list synchronizes, then renders the catalog', async () => {
    vi.useFakeTimers();
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(book), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ state: 'syncing' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify([{ id: 'book-1_0', bookId: 'book-1', index: 0, title: 'Chapter One', url: '/1', isVolume: false }]), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);

    const wrapper = mount(BookDetailView, {
      global: {
        plugins: [i18n],
        mocks: { $route: { params: { bookId: 'book-1' } }, $router: { replace: vi.fn() } },
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          FeatureScaffold: { template: '<main><slot /></main>' },
          BookCover: true,
          BookDetailSection: { template: '<section><slot name="body" /><slot /></section>' },
          BookDetailToc: { props: ['chapters'], template: '<div>{{ chapters.map((chapter) => chapter.title).join(",") }}</div>' },
          SourceRecoveryPanel: true,
          WebViewFailureHint: true,
        },
      },
    });
    await flushPromises();

    expect(wrapper.text()).toContain('Fixture Novel');
    expect(wrapper.text()).toContain('Synchronizing the chapter list…');
    expect(wrapper.text()).not.toContain('Chapter One');

    await vi.advanceTimersByTimeAsync(500);
    await flushPromises();

    expect(wrapper.text()).toContain('Chapter One');
    expect(wrapper.text()).not.toContain('Synchronizing the chapter list…');
  });

  it('shows the catalog failure and retries through the sync endpoint', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(book), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 'source_request_failed', error: 'All aggregate routes failed.' }), { status: 502, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ state: 'syncing' }), { status: 202, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);

    const wrapper = mount(BookDetailView, {
      global: {
        plugins: [i18n],
        mocks: { $route: { params: { bookId: 'book-1' } }, $router: { replace: vi.fn() } },
        stubs: {
          RouterLink: { template: '<a><slot /></a>' }, FeatureScaffold: { template: '<main><slot /></main>' },
          BookCover: true, BookDetailSection: { template: '<section><slot name="body" /><slot /></section>' },
          BookDetailToc: true, SourceRecoveryPanel: true, WebViewFailureHint: true,
          AppButton: { props: ['busy'], template: '<button @click="$emit(\'click\')"><slot /></button>' },
        },
      },
    });
    await flushPromises();

    expect(wrapper.text()).toContain('All aggregate routes failed.');
    const retry = wrapper.findAll('button').find((button) => button.text() === 'Retry chapter list');
    expect(retry).toBeDefined();
    await retry!.trigger('click');
    await flushPromises();

    expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/books/book-1/chapters/sync', expect.objectContaining({ method: 'POST' }));
  });
});
