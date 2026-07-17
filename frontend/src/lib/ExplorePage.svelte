<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ExploreApiError, addSearchResultToShelf, getExplorePage, listExploreSources,
    openExplore, updateExploreControl, type ExploreCatalog, type ExploreEntry,
    type ExploreSource, type SearchResult,
  } from '../api/client';
  import ExploreControls from './ExploreControls.svelte';
  import SearchResults from './SearchResults.svelte';
  import { categorySelection, classifyExploreError, selectedCategoryAfterRefresh } from './exploreState.js';

  let { go }: { go: (path: string) => void } = $props();
  let sources = $state<ExploreSource[]>([]);
  let sourceId = $state('');
  let catalog = $state<ExploreCatalog | null>(null);
  let categoryId = $state('');
  let results = $state<SearchResult[]>([]);
  let nextPage = $state(1);
  let exhausted = $state(false);
  let busy = $state('');
  let error = $state('');
  let retry = $state<null | (() => void)>(null);
  let adding = $state<string | null>(null);
  let diagnostics = $state<string[]>([]);
  let categoryCache = $state<Record<string, { results: SearchResult[]; nextPage: number; exhausted: boolean }>>({});
  let requestNumber = 0;
  let controller: AbortController | null = null;

  let controls = $derived(catalog?.entries.filter((entry) => ['text', 'button', 'toggle', 'select'].includes(entry.type)) || []);
  let categories = $derived(catalog?.entries.filter((entry) => entry.type === 'url') || []);

  onMount(() => {
    loadSources();
    return () => controller?.abort();
  });

  function begin(kind: string) {
    controller?.abort();
    controller = new AbortController();
    requestNumber += 1;
    busy = kind;
    error = '';
    retry = null;
    return { number: requestNumber, signal: controller.signal };
  }

  function finish(number: number) {
    if (number === requestNumber) busy = '';
  }

  function resetResults() {
    categoryId = '';
    results = [];
    nextPage = 1;
    exhausted = false;
    diagnostics = [];
    categoryCache = {};
  }

  function handleFailure(caught: unknown, retryAction: () => void, number: number) {
    if (number !== requestNumber || (caught instanceof DOMException && caught.name === 'AbortError')) return;
    const apiError = caught instanceof ExploreApiError ? caught : null;
    const recovery = classifyExploreError(apiError);
    if (recovery.kind === 'reopen' && sourceId) {
      catalog = null;
      resetResults();
      void openSource(sourceId);
      return;
    }
    if (recovery.kind === 'page') nextPage = recovery.page;
    error = caught instanceof Error ? caught.message : 'Explore request failed';
    retry = recovery.kind === 'stop' ? null : retryAction;
  }

  async function loadSources() {
    const request = begin('sources');
    try {
      const loaded = await listExploreSources(request.signal);
      if (request.number !== requestNumber) return;
      sources = loaded;
    } catch (caught) {
      handleFailure(caught, loadSources, request.number);
    } finally {
      finish(request.number);
    }
  }

  async function openSource(id: string) {
    sourceId = id;
    catalog = null;
    resetResults();
    if (!id) return;
    const request = begin('catalog');
    try {
      const opened = await openExplore(id, request.signal);
      if (request.number !== requestNumber) return;
      catalog = opened;
      diagnostics = opened.diagnostics.map((item) => item.message);
    } catch (caught) {
      handleFailure(caught, () => openSource(id), request.number);
    } finally {
      finish(request.number);
    }
  }

  async function updateControl(entry: ExploreEntry, value: string | null) {
    if (!catalog) return;
    const sessionId = catalog.sessionId;
    const request = begin('control');
    try {
      const refreshed = await updateExploreControl(sessionId, entry.id, value, request.signal);
      if (request.number !== requestNumber) return;
      const retained = selectedCategoryAfterRefresh(categoryId, refreshed.entries);
      catalog = refreshed;
      if (!retained) resetResults();
      else categoryId = retained;
      diagnostics = refreshed.diagnostics.map((item) => item.message);
    } catch (caught) {
      handleFailure(caught, () => openSource(sourceId), request.number);
    } finally {
      finish(request.number);
    }
  }

  async function loadPage(page: number, reset: boolean) {
    if (!catalog || !categoryId) return;
    const request = begin('page');
    try {
      const response = await getExplorePage(catalog.sessionId, categoryId, page, request.signal);
      if (request.number !== requestNumber) return;
      results = reset ? response.books : [...results, ...response.books];
      nextPage = response.nextPage;
      exhausted = response.exhausted;
      categoryCache = { ...categoryCache, [categoryId]: { results, nextPage, exhausted } };
      diagnostics = response.diagnostics.map((item) => item.message);
    } catch (caught) {
      handleFailure(caught, () => loadPage(nextPage, false), request.number);
    } finally {
      finish(request.number);
    }
  }

  function selectCategory(entry: ExploreEntry) {
    const selection = categorySelection(categoryId, entry.id, categoryCache);
    if (selection.kind === 'current') return;
    categoryId = entry.id;
    if (selection.kind === 'cached') {
      results = selection.state.results;
      nextPage = selection.state.nextPage;
      exhausted = selection.state.exhausted;
      return;
    }
    results = [];
    nextPage = 1;
    exhausted = false;
    void loadPage(1, true);
  }

  async function addToShelf(result: SearchResult) {
    adding = result.bookUrl;
    try {
      const book = await addSearchResultToShelf(result);
      go(`book?id=${book.id}`);
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Could not add this book';
    } finally {
      adding = null;
    }
  }
</script>

<div class="page" aria-busy={Boolean(busy)}>
  <h2>Explore</h2>
  <label class="source-picker">
    <span>Source</span>
    <select value={sourceId} disabled={busy !== ''} onchange={(event) => openSource(event.currentTarget.value)}>
      <option value="">Choose a source…</option>
      {#each sources as source (source.id)}
        <option value={source.id}>{source.name}{source.group ? ` · ${source.group}` : ''}</option>
      {/each}
    </select>
  </label>

  {#if busy === 'sources'}
    <p class="status" aria-live="polite">Loading sources…</p>
  {:else if sources.length === 0 && !error}
    <div class="empty">No Explore sources are enabled. <button onclick={() => go('sources')}>Open Sources</button></div>
  {/if}

  {#if catalog}
    <ExploreControls entries={controls} disabled={busy !== ''} onupdate={updateControl} />
    {#if categories.length > 0}
      <div class="categories" aria-label="Categories">
        {#each categories as entry (entry.id)}
          {#if entry.selectable}
            <button class:active={categoryId === entry.id} aria-pressed={categoryId === entry.id}
              disabled={busy !== ''} onclick={() => selectCategory(entry)}>{entry.title}</button>
          {:else if entry.title}
            <span class="category-heading">{entry.title}</span>
          {/if}
        {/each}
      </div>
    {:else if controls.length === 0}
      <p class="empty">This source has no Explore categories.</p>
    {/if}
  {/if}

  <div class="status" aria-live="polite">
    {#if busy === 'catalog'}Loading categories…
    {:else if busy === 'control'}Updating Explore…
    {:else if busy === 'page' && results.length === 0}Loading books…
    {:else if categoryId && results.length === 0 && exhausted}Nothing in this category yet.
    {:else if results.length > 0 && exhausted}That’s everything here.
    {/if}
  </div>
  {#if error}
    <div class="error" role="alert"><span>{error}</span>{#if retry}<button onclick={retry}>Retry</button>{/if}</div>
  {/if}
  {#if diagnostics.length > 0}
    <ul class="diagnostics">{#each diagnostics as message}<li>{message}</li>{/each}</ul>
  {/if}
  {#if results.length > 0}<SearchResults {results} {adding} onadd={addToShelf} />{/if}
  {#if categoryId && results.length > 0 && !exhausted}
    <button class="more" disabled={busy !== ''} onclick={() => loadPage(nextPage, false)}>Load more</button>
  {/if}
</div>

<style>
  .page { padding: 1rem; }
  h2 { font-size: 1.1rem; margin-bottom: 0.75rem; }
  .source-picker { display: flex; flex-direction: column; gap: 0.25rem; margin-bottom: 0.8rem; }
  .source-picker span { color: #666; font-size: 0.78rem; font-weight: 600; }
  select { min-height: 2.75rem; padding: 0.45rem 0.6rem; border: 1px solid var(--border); border-radius: 8px; background: var(--card-bg); color: var(--fg); font: inherit; }
  .categories { display: flex; align-items: center; flex-wrap: wrap; gap: 0.4rem; margin-bottom: 0.8rem; }
  .categories button, .more, .error button, .empty button { min-height: 2.5rem; padding: 0.45rem 0.75rem; border: 1px solid var(--border); border-radius: 9px; background: var(--card-bg); color: var(--fg); cursor: pointer; }
  .categories button.active { background: var(--accent); border-color: var(--accent); color: white; }
  .category-heading { width: 100%; margin-top: 0.25rem; font-size: 0.82rem; font-weight: 600; }
  .status { min-height: 1.2rem; margin-bottom: 0.5rem; color: #666; font-size: 0.82rem; }
  .empty { padding: 1rem 0; color: #666; }
  .error { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; margin-bottom: 0.7rem; color: #b42318; font-size: 0.88rem; }
  .diagnostics { margin: 0 0 0.7rem 1rem; color: #666; font-size: 0.8rem; }
  .more { width: 100%; margin-top: 0.8rem; }
  button:focus-visible, select:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  button:disabled, select:disabled { cursor: not-allowed; opacity: 0.6; }
</style>
