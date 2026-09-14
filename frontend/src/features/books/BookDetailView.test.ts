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
      title: 'Book details', description: 'Description', loading: 'Loading', loadFailed: 'Load failed', tocFailed: 'TOC failed', tocSyncing: 'Synchronizing the chapter list…', retryToc: 'Retry chapter list', notFound: 'Not found', back: 'Back', coverAlt: 'Cover of {name}', tocEntries: '{count} entries', progress: '{percent}% read', latest: 'Latest: {chapter}', currentSource: 'Current source: {source}', continue: 'Continue', remove: 'Remove', confirmRemoveTitle: 'Remove?', confirmRemoveDescription: 'Remove {name}?', cancel: 'Cancel', confirmRemove: 'Remove', confirmRemoveTXT: 'Delete managed original for {name}?', removed: 'Removed from your library', cleanupPending: 'File cleanup pending. You can retry.', retryCleanup: 'Retry file cleanup', synopsis: 'Synopsis', chapters: 'Chapters', noChapters: 'No chapters', showAll: 'Show all {count}',
    },
    reader: { toc: { readableSummary: '{readable} readable', summary: '{readable}/{total}', search: 'Search', searchPlaceholder: 'Search', clearSearch: 'Clear', ascending: 'Ascending', descending: 'Descending', jumpCurrent: 'Current', matches: '{count} matches', noMatches: 'No matches' } },
    sourceRecovery: { title: 'Sources', cleared: 'Cleared' },
  } },
});

const book = {
  id: 'book-1', name: 'Fixture Novel', author: 'Author', coverUrl: '', intro: '', kind: '', sourceId: 'source-1', sourceUrl: 'source-1', bookUrl: '/book', origin: 'Source', lastChapter: '', durChapterIndex: 0, durChapterPos: 0, totalChapterNum: 0, provider: 'booksource', contentRevision: 0, stateVersion: 0, alternateSources: [],
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
      .mockResolvedValueOnce(new Response(JSON.stringify(book), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ state: 'syncing' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ contentRevision: 1, chapters: [{ id: 'book-1_0', bookId: 'book-1', index: 0, title: 'Chapter One', url: '/1', isVolume: false }] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ...book, contentRevision: 1 }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ contentRevision: 1, chapters: [{ index: 0, title: 'Chapter One', isVolume: false }] }), { status: 200 }));
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
          BookDetailToc: { props: ['chapters', 'contentRevision'], template: '<div>{{ chapters.map((chapter) => chapter.title).join(",") }}</div>' },
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
    expect(wrapper.vm.catalogRevision).toBe(1);
    expect(wrapper.vm.book?.contentRevision).toBe(1);
    expect(wrapper.text()).not.toContain('Synchronizing the chapter list…');
  });

  it('shows the catalog failure and retries through the sync endpoint', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(book), { status: 200, headers: { 'Content-Type': 'application/json' } }))
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

    expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/books/book-1/chapters/sync', expect.objectContaining({ method: 'POST' }));
  });
});

it('renders provider-neutral details without requesting BookSource context', async () => {
  const item = { id: 'local', provider: 'fixture', name: 'Local publication', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', durChapterIndex: 0, durChapterPos: 0, totalChapterNum: 0, contentRevision: 2, stateVersion: 0 };
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url === '/api/books/local') return new Response(JSON.stringify(item), { status: 200 });
    if (url === '/api/books/local/chapters') return new Response(JSON.stringify({ chapters: [], contentRevision: 2 }), { status: 200 });
    throw new Error(`Unexpected request: ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
  const wrapper = mount(BookDetailView, { global: {
    plugins: [i18n], mocks: { $route: { params: { bookId: 'local' } } },
    stubs: { RouterLink: { template: '<a><slot /></a>' }, FeatureScaffold: { template: '<main><slot /></main>' }, BookCover: true, BookDetailSection: true, BookDetailToc: true, SourceRecoveryPanel: true },
  } });
  try {
    await flushPromises();
    expect(wrapper.text()).toContain('Local publication');
    expect(wrapper.find('source-recovery-panel-stub').exists()).toBe(false);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  } finally { wrapper.unmount(); }
});

it('keeps TXT removal warnings visible and retries cleanup without restoring a shelf row', async () => {
  const item = { id:'local',provider:'txt',name:'Local publication',author:'Author',coverUrl:'',intro:'',kind:'',lastChapter:'',durChapterIndex:0,durChapterPos:0,totalChapterNum:1,contentRevision:2,stateVersion:0 };
  let removals = 0;
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    if (url === '/api/books/local') return new Response(JSON.stringify(item));
    if (url === '/api/books/local/chapters') return new Response(JSON.stringify({chapters:[{index:0,title:'First',isVolume:false}],contentRevision:2}));
    if (url === '/api/books?id=local' && init?.method === 'DELETE') {
      removals++;
      if (removals === 2) return new Response(JSON.stringify({error:'Cleanup unavailable'}), {status:500});
      return new Response(JSON.stringify(removals === 1 ? {status:'removed',warnings:['txt_cleanup_pending']} : {status:'deleted'}));
    }
    throw new Error(`Unexpected request: ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
  const replace = vi.fn();
  const wrapper = mount(BookDetailView, {attachTo:document.body,global:{
    plugins:[i18n], mocks:{$route:{params:{bookId:'local'}},$router:{replace}},
    stubs:{RouterLink:{template:'<a><slot /></a>'},FeatureScaffold:{template:'<main><slot /></main>'},BookCover:true,BookDetailSection:true,BookDetailToc:true,SourceRecoveryPanel:true},
  }});
  try {
    await flushPromises();
    await wrapper.findAll('button').find(button=>button.text()==='Remove')!.trigger('click');
    expect(wrapper.text()).toContain('Delete managed original for Local publication?');
    await wrapper.findAll('button').filter(button=>button.text()==='Remove').at(-1)!.trigger('click');
    await flushPromises();
    expect(wrapper.text()).toContain('Removed from your library');
    expect(wrapper.text()).toContain('File cleanup pending');
    expect(document.activeElement).toBe(wrapper.get('h2').element);
    expect(wrapper.find('book-detail-toc-stub').exists()).toBe(false);
    expect(replace).not.toHaveBeenCalled();
    await wrapper.findAll('button').find(button=>button.text()==='Retry file cleanup')!.trigger('click');
    await flushPromises();
    expect(wrapper.text()).toContain('Cleanup unavailable');
    expect(wrapper.text()).toContain('File cleanup pending');
    await wrapper.findAll('button').find(button=>button.text()==='Retry file cleanup')!.trigger('click');
    await flushPromises();
    expect(removals).toBe(3);
    expect(replace).toHaveBeenCalledWith('/shelf');
  } finally { wrapper.unmount(); }
});
