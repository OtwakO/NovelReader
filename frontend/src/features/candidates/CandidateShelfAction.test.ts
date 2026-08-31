import { flushPromises, mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import CandidateShelfAction from './CandidateShelfAction.vue';

const messages = {
  candidate: {
    title: 'Finding source', help: 'First readable source wins', starting: 'Starting', finishing: 'Finishing',
    runningSummary: '{active} active · {completed}/{known}', failedSummary: 'No readable source · {completed}/{known}',
    counts: '{completed}/{known} · {active} active', winnerCounts: 'Winner · {active} draining · {skipped} skipped', cancelling: 'Cancelling', cancelFailed: 'Cancel failed',
    queued: '{count} waiting', skipped: '{count} skipped', moreFailed: '{count} more failed',
    stage: { book_info: 'Book info', toc: 'Contents', content: 'Chapter text' },
    state: { running: 'Checking', failed: 'Unavailable', verified: 'Readable', skipped: 'Not needed' },
  },
  search: { actions: { more: 'Scan {count} more sources', retry: 'Retry batch', restart: 'Restart search' }, results: {
    shelve: 'Add', retryAdd: 'Retry sources', retryCommit: 'Retry add', cancelShelving: 'Cancel', added: 'Added', cancelled: 'Cancelled',
    addFailed: 'Failed', disconnected: 'Disconnected',
  } },
};
const i18n = createI18n({ legacy: false, globalInjection: true, locale: 'en', messages: { en: messages } });
const result = { name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/book', sourceId: 'source', sourceUrl: 'source', sourceName: 'Primary' };

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;
  constructor(public url: string) { FakeEventSource.instances.push(this); }
  close() { this.closed = true; }
  emit(value: unknown) { this.onmessage?.({ data: JSON.stringify(value) } as MessageEvent); }
}

function snapshot(state = 'running') {
  return {
    id: 'operation', state, known: 5, completed: 1, active: 2, updatedAt: new Date().toISOString(),
    attempts: [
      { sourceName: 'Primary', sourceId: 'source', sourceUrl: 'source', bookUrl: '/book', state: 'running', stage: 'toc' },
      { sourceName: 'Alternate', sourceId: 'alternate', sourceUrl: 'alternate', bookUrl: '/alternate', state: 'running', stage: 'content' },
      { sourceName: 'Broken', sourceId: 'broken', sourceUrl: 'broken', bookUrl: '/broken', state: 'failed', stage: 'book_info' },
      { sourceName: 'Waiting', sourceId: 'waiting', sourceUrl: 'waiting', bookUrl: '/waiting', state: 'queued' },
      { sourceName: 'Waiting 2', sourceId: 'waiting-2', sourceUrl: 'waiting-2', bookUrl: '/waiting-2', state: 'queued' },
    ],
  };
}

function rememberedSnapshot(state = 'running') {
  return {
    id: 'operation', state, known: 1, completed: 0, active: state === 'running' ? 1 : 0, updatedAt: new Date().toISOString(),
    attempts: [{ sourceName: 'Primary', sourceId: 'source', sourceUrl: 'source', bookUrl: '/book', state: state === 'running' ? 'running' : 'verified', stage: 'content' }],
  };
}

beforeEach(() => {
  sessionStorage.clear();
  FakeEventSource.instances = [];
  vi.stubGlobal('EventSource', FakeEventSource);
  vi.stubGlobal('crypto', { randomUUID: () => 'book-id' });
});

describe('CandidateShelfAction', () => {
  it('shows authoritative shelf membership without starting candidate resolution', () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result: { ...result, shelfBookId: 'shelf-book' } } });
    expect(wrapper.text()).toContain('Added');
    expect(wrapper.find('.app-button').exists()).toBe(false);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('shows a collapsible source checklist while streamed progress continues', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(snapshot()), {
      status: 202, headers: { 'Content-Type': 'application/json' },
    })));
    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });

    await wrapper.get('button').trigger('click');
    await flushPromises();

    expect(FakeEventSource.instances[0]?.url).toContain('/candidate-resolutions/operation/events');
    expect(wrapper.get('.progress-toggle').attributes('aria-expanded')).toBe('true');
    expect(wrapper.text()).toContain('Primary');
    expect(wrapper.text()).toContain('Contents');
    expect(wrapper.text()).toContain('2 waiting');

    await wrapper.get('.progress-toggle').trigger('click');
    expect(wrapper.find('.progress-detail').exists()).toBe(false);

    FakeEventSource.instances[0]?.emit({ ...snapshot(), completed: 2, active: 1 });
    await wrapper.vm.$nextTick();
    expect(wrapper.get('.progress-toggle').text()).toContain('1 active · 2/5');
  });

  it('shows a verified winner separately while active checks drain and untouched sources are skipped', async () => {
    const winning = {
      ...snapshot(), active: 1, completed: 4,
      attempts: [
        { sourceName: 'Winner', sourceId: 'winner', sourceUrl: 'winner', state: 'verified', stage: 'content' },
        { sourceName: 'Draining', sourceId: 'draining', sourceUrl: 'draining', state: 'running', stage: 'toc' },
        { sourceName: 'Broken', sourceId: 'broken', sourceUrl: 'broken', state: 'failed', stage: 'content' },
        { sourceName: 'Unused', sourceId: 'unused', sourceUrl: 'unused', state: 'skipped' },
        { sourceName: 'Unused 2', sourceId: 'unused-2', sourceUrl: 'unused-2', state: 'skipped' },
      ],
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(winning), {
      status: 202, headers: { 'Content-Type': 'application/json' },
    })));

    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });
    await wrapper.get('button').trigger('click');
    await flushPromises();

    expect(wrapper.get('.progress-toggle').text()).toContain('Finishing');
    expect(wrapper.get('.verified').text()).toContain('Winner');
    expect(wrapper.get('.verified').text()).toContain('Readable');
    expect(wrapper.get('.active').text()).toContain('Draining');
    expect(wrapper.text()).toContain('2 skipped');
    expect(wrapper.text()).not.toContain('2 waiting');
  });

  it('renders the initial operation snapshot before attempts are initialized', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...snapshot(), attempts: null }), {
      status: 201, headers: { 'Content-Type': 'application/json' },
    })));

    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });
    await wrapper.get('button').trigger('click');
    await flushPromises();

    expect(wrapper.get('.progress-toggle').text()).toContain('2 active · 1/5');
    expect(wrapper.find('.attempt-list').exists()).toBe(false);
  });

  it('restores and reconnects an operation after remount', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['source\u0000/book']: 'operation' }));
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(rememberedSnapshot()), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    })));

    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });
    await flushPromises();

    expect(wrapper.get('.progress-toggle').text()).toContain('1 active · 0/1');
    expect(FakeEventSource.instances[0]?.url).toContain('/candidate-resolutions/operation/events');
    expect(wrapper.find('.progress-detail').exists()).toBe(false);
  });

  it('discards a remembered operation when the Search result has newer source bindings', async () => {
    const enriched = { ...result, alternateSources: [{ sourceId: 'new-source', sourceUrl: 'new-source', bookUrl: '/new-book', sourceName: 'New' }] };
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['source\u0000/book']: 'operation' }));
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(rememberedSnapshot('verified')), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    })));

    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result: enriched } });
    await flushPromises();

    expect(wrapper.find('.progress-toggle').exists()).toBe(false);
    expect(wrapper.get('button').text()).toBe('Add');
    expect(sessionStorage.length).toBe(0);
  });

  it('drops a finished operation when later Search batches enrich the mounted result', async () => {
    const verified = {
      ...rememberedSnapshot('verified'), automaticCommit: false,
      preview: { book: {}, chapters: [], selection: {} },
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(verified), {
      status: 202, headers: { 'Content-Type': 'application/json' },
    })));
    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });
    await wrapper.get('button').trigger('click');
    await flushPromises();
    expect(wrapper.find('.progress-toggle').exists()).toBe(false);

    await wrapper.setProps({ result: { ...result, alternateSources: [{ sourceId: 'new-source', sourceUrl: 'new-source', bookUrl: '/new-book', sourceName: 'New' }] } });
    await flushPromises();

    expect(wrapper.get('button').text()).toBe('Add');
    expect((wrapper.vm as unknown as { snapshot: unknown }).snapshot).toBeNull();
    expect(sessionStorage.length).toBe(0);
  });

  it('keeps following an immediately verified automatic start', async () => {
    const verified = {
      ...snapshot('verified'), active: 0, completed: 1, automaticCommit: true,
      preview: { book: {}, chapters: [], selection: {} },
      attempts: [{ sourceName: 'Winner', sourceId: 'winner', sourceUrl: 'winner', state: 'verified', stage: 'content' }],
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(verified), { status: 202, headers: { 'Content-Type': 'application/json' } })));

    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });
    await wrapper.get('button').trigger('click');
    await flushPromises();

    expect(wrapper.get('.progress-toggle').text()).toContain('Finishing');
    expect(FakeEventSource.instances[0]?.url).toContain('/candidate-resolutions/operation/events');
  });

  it('reuses a verified detail operation for Add instead of showing an endless finishing state', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['source\u0000/book']: 'operation' }));
    const verified = {
      ...snapshot('verified'), active: 0, completed: 3, automaticCommit: false,
      preview: { book: {}, chapters: [], selection: {} },
      attempts: [{ sourceName: 'Primary', sourceId: 'source', sourceUrl: 'source', bookUrl: '/book', state: 'verified', stage: 'content' }],
    };
    const committed = { ...verified, state: 'committed', storedBook: { id: 'stored' } };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(verified), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(committed), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);

    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });
    await flushPromises();

    expect(wrapper.find('.progress-toggle').exists()).toBe(false);
    expect(wrapper.get('button').text()).toBe('Add');
    await wrapper.get('button').trigger('click');
    await flushPromises();

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(String(fetchMock.mock.calls[1]?.[0])).toContain('/candidate-resolutions/operation/shelve');
    expect(wrapper.get('.completed-status').text()).toContain('Added');
    expect(sessionStorage.getItem('novelreader.candidate-operations.v1')).toContain('stored');
  });

  it('keeps cancellation pending until the backend publishes cancelled', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(snapshot()), { status: 202, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);
    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });

    await wrapper.get('button').trigger('click');
    await flushPromises();
    await wrapper.get('.progress-detail button').trigger('click');
    await flushPromises();

    expect(wrapper.text()).toContain('Cancelling');
    expect(wrapper.find('.progress-toggle').exists()).toBe(true);

    const stream = FakeEventSource.instances.at(-1);
    stream?.emit({ ...snapshot('cancelled'), active: 0, completed: 3 });
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect((wrapper.vm as unknown as { snapshot: unknown }).snapshot).toBeNull();
    expect(wrapper.find('.progress-toggle').exists()).toBe(false);
    expect(wrapper.get('button').text()).toBe('Add');
    expect(wrapper.text()).toContain('Cancelled');
    expect(sessionStorage.length).toBe(0);
  });

  it('separates exhausted status from the actionable retry control after restoration', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['source\u0000/book']: 'operation' }));
    const exhausted = {
      ...rememberedSnapshot('exhausted'), active: 0, completed: 3, known: 3,
      attempts: [
        { sourceName: 'One', sourceId: 'source', sourceUrl: 'source', bookUrl: '/book', state: 'failed', stage: 'content' },
        { sourceName: 'Two', sourceId: 'two', sourceUrl: 'two', bookUrl: '/two', state: 'failed', stage: 'toc' },
        { sourceName: 'Three', sourceId: 'three', sourceUrl: 'three', bookUrl: '/three', state: 'failed', stage: 'book_info' },
      ],
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(exhausted), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    })));

    const failedResult = {
      ...result,
      alternateSources: [
        { sourceId: 'two', sourceUrl: 'two', bookUrl: '/two', sourceName: 'Two' },
        { sourceId: 'three', sourceUrl: 'three', bookUrl: '/three', sourceName: 'Three' },
      ],
    };
    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result: failedResult } });
    await flushPromises();

    expect(wrapper.get('.app-button').text()).toBe('Retry sources');
    expect(wrapper.get('.failure-summary').text()).toContain('No readable source · 3/3');
    expect(wrapper.find('.progress-toggle').exists()).toBe(false);
    expect(wrapper.find('.progress-detail').exists()).toBe(true);
  });

  it('continues the normal Search batch after exhaustion, then returns to Add', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['source\u0000/book']: 'operation' }));
    const exhausted = {
      ...rememberedSnapshot('exhausted'), active: 0, completed: 1, known: 1,
      attempts: [{ sourceName: 'Primary', sourceId: 'source', sourceUrl: 'source', bookUrl: '/book', state: 'failed', stage: 'content' }],
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(exhausted), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    })));

    const wrapper = mount(CandidateShelfAction, {
      global: { plugins: [i18n] },
      props: { result, canContinueSearch: true, continueSearchCount: 50, searchScanning: false },
    });
    await flushPromises();

    expect(wrapper.get('.app-button').text()).toContain('50');
    await wrapper.get('.app-button').trigger('click');
    expect(wrapper.emitted('continue-search')).toHaveLength(1);
    expect(sessionStorage.length).toBe(0);

    await wrapper.setProps({ searchScanning: true });
    expect(wrapper.get('.app-button').text()).toContain('50');
    await wrapper.setProps({ searchScanning: false });

    expect(wrapper.get('.app-button').text()).toBe('Add');
    expect(wrapper.find('.failure-summary').exists()).toBe(false);
  });

  it('keeps Search recovery aligned after a continued batch disconnects', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['source\u0000/book']: 'operation' }));
    const exhausted = {
      ...rememberedSnapshot('exhausted'), active: 0, completed: 1, known: 1,
      attempts: [{ sourceName: 'Primary', sourceId: 'source', sourceUrl: 'source', bookUrl: '/book', state: 'failed', stage: 'content' }],
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(exhausted), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    })));
    const wrapper = mount(CandidateShelfAction, {
      global: { plugins: [i18n] },
      props: { result, canContinueSearch: true, continueSearchCount: 50 },
    });
    await flushPromises();
    await wrapper.get('.app-button').trigger('click');
    await wrapper.setProps({ searchScanning: true });
    await wrapper.setProps({ searchScanning: false, searchRetryRequired: true });

    expect(wrapper.get('.app-button').text()).toBe('Retry batch');
    expect(wrapper.find('.failure-summary').exists()).toBe(false);
    await wrapper.get('.app-button').trigger('click');
    expect(wrapper.emitted('retry-search')).toHaveLength(1);

    await wrapper.setProps({ searchRetryRequired: false, searchScanning: true });
    await wrapper.setProps({ searchScanning: false });
    expect(wrapper.get('.app-button').text()).toBe('Add');
  });

  it('keeps Search recovery aligned when continued scanning becomes stale', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['source\u0000/book']: 'operation' }));
    const exhausted = {
      ...rememberedSnapshot('exhausted'), active: 0, completed: 1, known: 1,
      attempts: [{ sourceName: 'Primary', sourceId: 'source', sourceUrl: 'source', bookUrl: '/book', state: 'failed', stage: 'content' }],
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(exhausted), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    })));
    const wrapper = mount(CandidateShelfAction, {
      global: { plugins: [i18n] },
      props: { result, canContinueSearch: true, continueSearchCount: 50 },
    });
    await flushPromises();
    await wrapper.get('.app-button').trigger('click');
    await wrapper.setProps({ searchScanning: true });
    await wrapper.setProps({ searchScanning: false, searchRestartRequired: true });

    expect(wrapper.get('.app-button').text()).toBe('Restart search');
    await wrapper.get('.app-button').trigger('click');
    expect(wrapper.emitted('restart-search')).toHaveLength(1);
  });

  it('retries known sources when Search has no additional batch', async () => {
    sessionStorage.setItem('novelreader.candidate-operations.v1', JSON.stringify({ ['source\u0000/book']: 'operation' }));
    const exhausted = {
      ...rememberedSnapshot('exhausted'), active: 0, completed: 1, known: 1,
      attempts: [{ sourceName: 'Primary', sourceId: 'source', sourceUrl: 'source', bookUrl: '/book', state: 'failed', stage: 'content' }],
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(exhausted), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    })));
    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });
    await flushPromises();
    expect(wrapper.get('.app-button').text()).toBe('Retry sources');
  });

  it('retries a failed automatic commit without starting another source operation', async () => {
    const failed = { ...snapshot('failed'), active: 0, commitPending: true, preview: { book: {}, chapters: [], selection: {} } };
    const committed = { ...failed, state: 'committed', commitPending: false, storedBook: { id: 'stored' } };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(failed), { status: 202, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(committed), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });

    await wrapper.get('button').trigger('click');
    await flushPromises();
    await wrapper.get('.app-button').trigger('click');
    await flushPromises();

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(String(fetchMock.mock.calls[1]?.[0])).toContain('/candidate-resolutions/operation/shelve');
    expect(wrapper.get('.completed-status').text()).toContain('Added');
  });

  it('removes the shelf action after a committed snapshot', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(snapshot()), {
      status: 202, headers: { 'Content-Type': 'application/json' },
    })));
    const wrapper = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });

    await wrapper.get('button').trigger('click');
    await flushPromises();
    FakeEventSource.instances[0]?.emit({ ...snapshot('committed'), active: 0, completed: 3, storedBook: { id: 'stored' } });
    await wrapper.vm.$nextTick();

    expect(wrapper.find('button').exists()).toBe(false);
    expect(wrapper.get('.completed-status').text()).toContain('Added');

    wrapper.unmount();
    const restored = mount(CandidateShelfAction, { global: { plugins: [i18n] }, props: { result } });
    await restored.vm.$nextTick();
    expect(restored.find('button').exists()).toBe(false);
    expect(restored.get('.completed-status').text()).toContain('Added');
  });
});
