import { flushPromises, mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ReaderView from './ReaderView.vue';

const i18n = createI18n({
  legacy: false, globalInjection: true, locale: 'en',
  messages: { en: { reader: { title: 'Reader', back: 'Book details', readingArea: 'Reading area', loading: 'Loading chapter…', retryCatalog: 'Retry chapter list', recover: 'Switch reading source', bookmarks: { title: 'Bookmarks' }, settings: { wakeLockUnavailable: 'Unavailable' }, errors: { load: 'Load failed', noReadable: 'No readable chapters', chapter: 'Chapter unavailable' } } } },
});

const book = { id: 'book-1', name: 'Fixture Novel', author: 'Author', coverUrl: '', intro: '', kind: '', sourceId: 'source-1', sourceUrl: 'source-1', bookUrl: '/book', origin: 'Source', lastChapter: '', durChapterIndex: 0, durChapterPos: 0, totalChapterNum: 0, stateVersion: 0, alternateSources: [] };

afterEach(() => { vi.unstubAllGlobals(); });

describe('ReaderView catalog failure', () => {
  it('shows the failure with retry and source recovery actions', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/system/chinese-conversion') return new Response(JSON.stringify({ available: false, modes: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      if (url === '/api/books/book-1') return new Response(JSON.stringify(book), { status: 200, headers: { 'Content-Type': 'application/json' } });
      if (url === '/api/books/book-1/chapters/sync' && init?.method === 'POST') return new Response(JSON.stringify({ state: 'syncing' }), { status: 202, headers: { 'Content-Type': 'application/json' } });
      if (url === '/api/books/book-1/chapters') return new Response(JSON.stringify({ code: 'source_request_failed', error: 'All aggregate routes failed.' }), { status: 502, headers: { 'Content-Type': 'application/json' } });
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    const wrapper = mount(ReaderView, {
      global: {
        plugins: [i18n],
        mocks: { $route: { params: { bookId: 'book-1' }, query: {} }, $router: { push: vi.fn(), replace: vi.fn() } },
        stubs: {
          RouterLink: { template: '<a><slot /></a>' }, ReaderBookmarksSheet: true, ReaderControlIcon: true,
          ReaderSettingsSheet: true, ReaderSourceSheet: true, ReaderTocSheet: true,
          AppButton: { props: ['busy'], template: '<button @click="$emit(\'click\')"><slot /></button>' },
        },
      },
    });
    await flushPromises();

    expect(wrapper.text()).toContain('All aggregate routes failed.');
    expect(wrapper.text()).toContain('Retry chapter list');
    expect(wrapper.text()).toContain('Switch reading source');

    const retry = wrapper.findAll('button').find((button) => button.text() === 'Retry chapter list');
    await retry!.trigger('click');
    await flushPromises();
    expect(fetchMock).toHaveBeenCalledWith('/api/books/book-1/chapters/sync', expect.objectContaining({ method: 'POST' }));
  });
});
