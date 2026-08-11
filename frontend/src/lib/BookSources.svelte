<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    mergeBookSources, searchBooksBatchStream, switchBookSource,
    type AltSource, type Book, type SearchResult,
  } from '../api/client';
  import { matchesLogicalBook } from './bookIdentity.mjs';
  import { setProgressVersion, waitForProgressWrites } from './progressWriter';

  let {
    bookId,
    book = $bindable(),
    onchapterschanged,
  }: {
    bookId: string;
    book: Book;
    onchapterschanged: () => Promise<void> | void;
  } = $props();

  let expanded = $state(false);
  let selected = $state('');
  let switching = $state(false);
  let searching = $state(false);
  let message = $state('');
  let checked = $state(0);
  let eligible = $state(0);
  let cursor = $state('');
  let hasMore = $state(false);
  let sourceFailures = $state(0);
  let eventSource: EventSource | null = null;
  let mergeQueue = Promise.resolve();

  onDestroy(() => eventSource?.close());

  let sources = $derived([
    { sourceUrl: book.sourceUrl, bookUrl: book.bookUrl, sourceName: book.origin || book.sourceUrl, current: true },
    ...(book.alternateSources || []).map((source) => ({ ...source, current: false })),
  ]);

  function startSearch(startCursor = '') {
    eventSource?.close();
    searching = true;
    message = '';
    if (!startCursor) {
      checked = 0;
      eligible = 0;
      cursor = '';
      hasMore = false;
      sourceFailures = 0;
    }
    eventSource = searchBooksBatchStream(book.name, {
      cursor: startCursor, batchSize: 50, concurrency: 8,
    }, {
      onStart: (event) => {
        checked = event.offset;
        eligible = event.eligible;
      },
      onResult: async (_sourceId, items, nextChecked) => {
        checked = nextChecked;
        const matches = items.filter((item) => matchesLogicalBook(book, item));
        if (matches.length === 0) return;
        const discovered: AltSource[] = matches.flatMap((item: SearchResult) => [
          { sourceUrl: item.sourceUrl, bookUrl: item.bookUrl, sourceName: item.sourceName },
          ...(item.alternateSources || []),
        ]);
        mergeQueue = mergeQueue.then(async () => {
          try {
            book = await mergeBookSources(bookId, discovered);
          } catch (caught) {
            message = caught instanceof Error ? caught.message : 'Could not save discovered sources.';
          }
        });
        await mergeQueue;
      },
      onSourceError: (_sourceId, _error, nextChecked) => {
        checked = nextChecked;
        sourceFailures += 1;
      },
      onDone: (event) => {
        checked = event.checked;
        eligible = event.eligible;
        cursor = event.nextCursor || '';
        hasMore = event.hasMore;
        void mergeQueue.finally(() => {
          searching = false;
          eventSource = null;
        });
      },
      onStale: (staleMessage) => {
        message = staleMessage;
        searching = false;
        cursor = '';
        eventSource = null;
      },
      onDisconnect: () => {
        message = 'Source search was interrupted. Try this batch again.';
        searching = false;
        eventSource = null;
      },
    });
  }

  async function switchSource() {
    const source = sources[Number(selected)];
    if (!source || source.current || switching) return;
    switching = true;
    message = '';
    try {
      await waitForProgressWrites(bookId);
      const result = await switchBookSource(bookId, source.sourceUrl, source.bookUrl);
      book = result.book;
      setProgressVersion(bookId, result.book.stateVersion);
      selected = '';
      await onchapterschanged();
      message = result.mapping === 'title'
        ? 'Source switched at the matching chapter.'
        : 'Source switched using the nearest chapter index.';
    } catch (caught) {
      message = caught instanceof Error ? caught.message : 'Could not switch source.';
    } finally {
      switching = false;
    }
  }
</script>

<section class="book-sources" aria-labelledby="book-sources-heading">
  <div class="section-heading">
    <div>
      <h3 id="book-sources-heading">Book sources</h3>
      <p>Current: <strong>{book.origin || book.sourceUrl}</strong></p>
    </div>
    <button type="button" class="toggle" aria-expanded={expanded} onclick={() => expanded = !expanded}>
      {expanded ? 'Hide' : `${sources.length} source${sources.length === 1 ? '' : 's'}`}
    </button>
  </div>

  {#if expanded}
    <div class="source-actions">
      <select bind:value={selected} aria-label="Book source" disabled={switching}>
        <option value="">Select a source</option>
        {#each sources as source, index}
          <option value={String(index)} disabled={source.current}>
            {source.sourceName || source.sourceUrl}{source.current ? ' · Current' : ''}
          </option>
        {/each}
      </select>
      <button type="button" disabled={!selected || switching} onclick={switchSource}>
        {switching ? 'Switching…' : 'Switch'}
      </button>
    </div>

    <div class="discovery">
      <button type="button" disabled={searching} onclick={() => startSearch('')}>
        {searching ? 'Searching sources…' : 'Find more sources'}
      </button>
      {#if checked > 0}<span>{checked} of {eligible} checked{sourceFailures ? ` · ${sourceFailures} failed` : ''}</span>{/if}
      {#if hasMore && !searching}<button type="button" onclick={() => startSearch(cursor)}>Search 50 more</button>{/if}
    </div>
  {/if}

  {#if message}<p class="message" role="status">{message}</p>{/if}
</section>

<style>
  .book-sources { margin-bottom: 1rem; padding: 0.9rem; border: 1px solid var(--border); border-radius: 10px; background: var(--card-bg); }
  .section-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
  h3 { margin: 0; font-size: 0.95rem; }
  p { margin: 0.25rem 0 0; color: #666; font-size: 0.82rem; overflow-wrap: anywhere; }
  .toggle { border: 1px solid var(--border); background: transparent; color: inherit; }
  .source-actions, .discovery { display: flex; gap: 0.5rem; align-items: center; margin-top: 0.75rem; flex-wrap: wrap; }
  select { min-width: 0; flex: 1 1 15rem; padding: 0.55rem; border: 1px solid var(--border); border-radius: 7px; background: var(--card-bg); color: inherit; }
  button { padding: 0.55rem 0.75rem; border: 0; border-radius: 7px; background: var(--accent); color: white; cursor: pointer; }
  button:disabled, select:disabled { opacity: 0.55; cursor: default; }
  .discovery span { color: #777; font-size: 0.78rem; }
  .message { margin-top: 0.65rem; }
  button:focus-visible, select:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
