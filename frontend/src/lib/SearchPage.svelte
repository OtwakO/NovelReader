<script lang="ts">
  import {
    searchBooksBatchStream, addSearchResultToShelf, getChapters, getChapterContent,
    switchBookSource, type AltSource, type Book, type SearchResult,
  } from '../api/client';
  import SearchControls from './SearchControls.svelte';
  import SearchResults from './SearchResults.svelte';
  import SourceRecovery from './SourceRecovery.svelte';
  import { alternateSourceOptions, validateReadableBook } from './bookReadability.mjs';
  import SearchStatus from './SearchStatus.svelte';
  import { mergeSearchResults } from './searchResults.js';
  import {
    loadSearchPreferences, requestedConcurrency, saveSearchPreferences,
    type SearchIntensity,
  } from './searchPreferences';

  let { go }: { go: (path: string) => void } = $props();

  const STORAGE_KEY = 'nr_search_state';
  const STATE_VERSION = 2;

  let query = $state('');
  let searchedQuery = $state('');
  let results = $state<SearchResult[]>([]);
  let searching = $state(false);
  let checked = $state(0);
  let eligible = $state(0);
  let committedOffset = $state(0);
  let cursor = $state('');
  let retryCursor = $state('');
  let hasMore = $state(false);
  let retryRequired = $state(false);
  let restartRequired = $state(false);
  let batchSourceIds = $state<string[]>([]);
  let sourceFailures = $state(0);
  let effectiveConcurrency = $state(0);
  let activeBatchSize = $state(50);
  let activeConcurrency = $state(8);
  let error = $state('');
  let storageWarning = $state('');
  let es = $state<EventSource | null>(null);
  let initialized = $state(false);

  let batchSize = $state(50);
  let intensity = $state<SearchIntensity>('balanced');
  let advancedConcurrency = $state(8);

  $effect(() => {
    if (initialized) return;
    initialized = true;
    const preferences = loadSearchPreferences();
    batchSize = preferences.batchSize;
    intensity = preferences.intensity;
    advancedConcurrency = preferences.advancedConcurrency;

    const saved = sessionStorage.getItem(STORAGE_KEY);
    if (!saved) return;
    try {
      const state = JSON.parse(saved);
      if (!Array.isArray(state.results)) throw new Error('invalid saved results');
      query = typeof state.query === 'string' ? state.query : '';
      searchedQuery = typeof state.searchedQuery === 'string' ? state.searchedQuery : query;
      results = state.results;
      if (state.version === STATE_VERSION) {
        const numbers = [state.checked, state.eligible, state.committedOffset, state.sourceFailures, state.activeBatchSize, state.activeConcurrency];
        if (!Array.isArray(state.batchSourceIds) || numbers.some((value) => !Number.isFinite(value))) {
          throw new Error('invalid saved search state');
        }
        checked = Math.max(0, state.checked);
        eligible = Math.max(0, state.eligible);
        committedOffset = Math.max(0, state.committedOffset);
        cursor = typeof state.cursor === 'string' ? state.cursor : '';
        retryCursor = typeof state.retryCursor === 'string' ? state.retryCursor : '';
        hasMore = Boolean(state.hasMore);
        retryRequired = Boolean(state.retryRequired || state.inFlight);
        restartRequired = Boolean(state.restartRequired);
        batchSourceIds = state.batchSourceIds.filter((value: unknown) => typeof value === 'string');
        sourceFailures = Math.max(0, state.sourceFailures);
        activeBatchSize = Math.min(500, Math.max(1, state.activeBatchSize));
        activeConcurrency = Math.max(1, state.activeConcurrency);
      } else {
        checked = Number.isFinite(state.sourcesDone) ? Math.max(0, state.sourcesDone) : 0;
        restartRequired = results.length > 0;
      }
    } catch {
      sessionStorage.removeItem(STORAGE_KEY);
    }
  });

  $effect(() => () => {
    if (es) {
      es.close();
      es = null;
      searching = false;
      retryRequired = true;
      saveState();
    }
  });

  function preferences() {
    return { batchSize, intensity, advancedConcurrency };
  }

  function persistPreferences() {
    saveSearchPreferences(preferences());
  }

  function saveState() {
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify({
        version: STATE_VERSION,
        query, searchedQuery, results, checked, eligible, committedOffset,
        cursor, retryCursor, hasMore, retryRequired, restartRequired, inFlight: searching,
        batchSourceIds, sourceFailures, activeBatchSize, activeConcurrency,
      }));
      storageWarning = '';
    } catch {
      storageWarning = 'Search state could not be saved in this tab.';
    }
  }

  function handleSearch(event: Event) {
    event.preventDefault();
    const nextQuery = query.trim();
    if (!nextQuery) return;

    if (es) es.close();
    searchedQuery = nextQuery;
    results = [];
    checked = 0;
    eligible = 0;
    committedOffset = 0;
    cursor = '';
    retryCursor = '';
    hasMore = false;
    retryRequired = false;
    restartRequired = false;
    batchSourceIds = [];
    sourceFailures = 0;
    error = '';
    sessionStorage.removeItem(STORAGE_KEY);
    startBatch('', batchSize, requestedConcurrency(preferences()), false);
  }

  function startBatch(startCursor: string, size: number, concurrency: number, preservePartial: boolean) {
    if (es) es.close();
    if (!preservePartial) batchSourceIds = [];
    activeBatchSize = size;
    activeConcurrency = concurrency;
    retryCursor = startCursor;
    searching = true;
    retryRequired = false;
    restartRequired = false;
    error = '';

    es = searchBooksBatchStream(searchedQuery, {
      cursor: startCursor, batchSize: size, concurrency,
    }, {
      onStart: (event) => {
        committedOffset = event.offset;
        eligible = event.eligible;
        retryCursor = event.retryCursor;
        effectiveConcurrency = event.effectiveConcurrency;
        checked = committedOffset + batchSourceIds.length;
        saveState();
      },
      onResult: (sourceId, items) => {
        markSourceChecked(sourceId);
        results = mergeSearchResults(results, items, searchedQuery);
        saveState();
      },
      onSourceError: (sourceId) => {
        markSourceChecked(sourceId);
        saveState();
      },
      onDone: (event) => {
        searching = false;
        es = null;
        if (event.complete) {
          committedOffset = event.checked;
          checked = event.checked;
          cursor = event.nextCursor || '';
          retryCursor = '';
          hasMore = event.hasMore;
          retryRequired = false;
          batchSourceIds = [];
          sourceFailures += event.sourceFailures;
        } else {
          retryRequired = true;
        }
        saveState();
      },
      onStale: (message) => {
        searching = false;
        es = null;
        restartRequired = true;
        error = message;
        saveState();
      },
      onDisconnect: () => {
        searching = false;
        es = null;
        retryRequired = true;
        error = 'Search connection was interrupted. Retry this batch.';
        saveState();
      },
    });
  }

  function markSourceChecked(sourceId: string) {
    if (!batchSourceIds.includes(sourceId)) batchSourceIds = [...batchSourceIds, sourceId];
    checked = committedOffset + batchSourceIds.length;
  }

  function cancelSearch() {
    if (es) {
      es.close();
      es = null;
    }
    searching = false;
    retryRequired = true;
    error = '';
    saveState();
  }

  function retryBatch() {
    startBatch(retryCursor, activeBatchSize, activeConcurrency, true);
  }

  function searchMore() {
    startBatch(cursor, batchSize, requestedConcurrency(preferences()), false);
  }

  function restartSearch() {
    query = searchedQuery || query;
    handleSearch(new Event('submit'));
  }

  let adding = $state<string | null>(null);
  let recovery = $state<{
    book: Book;
    failedSource: string;
    error: string;
    sources: AltSource[];
  } | null>(null);
  let tryingSource = $state<string | null>(null);

  async function addToShelf(result: SearchResult) {
    adding = result.bookUrl;
    recovery = null;
    try {
      const book = await addSearchResultToShelf(result);
      try {
        await validateReadableBook(book.id, { getChapters, getChapterContent });
        go(`book?id=${book.id}`);
      } catch (caught: unknown) {
        recovery = {
          book,
          failedSource: result.sourceName || 'Selected source',
          error: caught instanceof Error ? caught.message : 'This source could not be read.',
          sources: alternateSourceOptions(result),
        };
      }
    } catch (caught: unknown) {
      alert('Failed: ' + (caught instanceof Error ? caught.message : 'Could not add this book'));
    } finally {
      adding = null;
    }
  }

  async function tryAlternateSource(source: AltSource) {
    if (!recovery) return;
    tryingSource = source.bookUrl;
    try {
      const switched = await switchBookSource(recovery.book.id, source.sourceUrl, source.bookUrl);
      await validateReadableBook(switched.book.id, { getChapters, getChapterContent });
      go(`book?id=${switched.book.id}`);
    } catch (caught: unknown) {
      recovery = {
        ...recovery,
        failedSource: source.sourceName || 'Selected source',
        error: caught instanceof Error ? caught.message : 'This source could not be read.',
        sources: recovery.sources.filter((candidate) => candidate.sourceUrl !== source.sourceUrl || candidate.bookUrl !== source.bookUrl),
      };
    } finally {
      tryingSource = null;
    }
  }

</script>

<div class="page">
  <form class="search-bar" onsubmit={handleSearch}>
    <input type="search" bind:value={query} placeholder="Search books..." aria-label="Book title" />
    {#if searching}
      <button type="button" class="cancel-btn" onclick={cancelSearch}>Stop</button>
    {:else}
      <button type="submit">Search</button>
    {/if}
  </form>

  <SearchControls bind:batchSize bind:intensity bind:advancedConcurrency onchange={persistPreferences} />

  {#if searchedQuery}
    <SearchStatus
      {checked} {eligible} resultCount={results.length} {searching}
      effectiveConcurrency={effectiveConcurrency || activeConcurrency}
      {sourceFailures} {error} {storageWarning} {restartRequired}
      {retryRequired} {hasMore} {batchSize}
      onrestart={restartSearch} onretry={retryBatch} onmore={searchMore}
    />
  {/if}

  {#if recovery}
    <SourceRecovery
      bookName={recovery.book.name}
      trying={tryingSource}
      failedSource={recovery.failedSource}
      error={recovery.error}
      sources={recovery.sources}
      ontry={tryAlternateSource}
      onclose={() => recovery = null}
    />
  {/if}

  {#if results.length > 0}
    <SearchResults {results} {adding} onadd={addToShelf} />
  {/if}
</div>

<style>
  .page { padding: 1rem; }
  .search-bar { display: flex; gap: 0.5rem; margin-bottom: 0.75rem; }
  .search-bar input {
    flex: 1; min-width: 0; padding: 0.6rem 0.8rem; border: 1px solid var(--border);
    border-radius: 10px; font-size: 1rem; background: var(--card-bg);
  }
  .search-bar button {
    background: var(--accent); color: white; border: none;
    padding: 0.6rem 1rem; border-radius: 10px; font-size: 0.9rem; cursor: pointer;
  }
  .search-bar .cancel-btn { background: #b42318; }
  button:focus-visible, input:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
