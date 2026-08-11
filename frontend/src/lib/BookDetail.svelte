<script lang="ts">
  import { getBook, getChapters, switchBookSource, type AltSource, type Book, type Chapter } from '../api/client';
  import { resolveChapterIndex } from './readingProgress.js';
  import { setProgressVersion, waitForProgressWrites } from './progressWriter';
  import BookSourceSwitcher from './BookSourceSwitcher.svelte';

  let { bookId, go }: { bookId: string; go: (path: string) => void } = $props();

  let book = $state<Book | null>(null);
  let chapters = $state<Chapter[]>([]);
  let loading = $state(true);
  let error = $state('');
  let switching = $state(false), switchMessage = $state('');
  let loadedBookId = '';
  let requestGeneration = 0;
  let continueIndex = $derived(book ? resolveChapterIndex(chapters, undefined, book.durChapterIndex) : null);

  $effect(() => {
    const id = bookId;
    if (!id) {
      requestGeneration += 1;
      loadedBookId = '';
      book = null;
      chapters = [];
      error = '';
      loading = false;
      return;
    }
    if (id !== loadedBookId) {
      loadedBookId = id;
      const generation = ++requestGeneration;
      void load(id, generation);
    }
  });

  async function load(id: string, generation: number) {
    loading = true;
    error = '';
    try {
      const nextBook = await getBook(id);
      if (generation !== requestGeneration) return;
      const nextChapters = await getChapters(id);
      if (generation !== requestGeneration) return;
      book = nextBook;
      chapters = nextChapters;
      setProgressVersion(id, nextBook.stateVersion);
      switchMessage = '';
    } catch (err) {
      if (generation !== requestGeneration) return;
      book = null;
      chapters = [];
      error = err instanceof Error ? err.message : 'Failed to load book';
    } finally {
      if (generation === requestGeneration) loading = false;
    }
  }

  async function switchSource(alternate: AltSource) {
    if (!book || switching) return;
    switching = true;
    switchMessage = '';
    try {
      await waitForProgressWrites(bookId);
      const result = await switchBookSource(bookId, alternate.sourceUrl, alternate.bookUrl);
      book = result.book;
      setProgressVersion(bookId, result.book.stateVersion);
      chapters = await getChapters(bookId);
      switchMessage = result.mapping === 'title'
        ? 'Source switched at the matching chapter.'
        : 'Source switched using the nearest chapter index.';
    } catch (err) {
      switchMessage = err instanceof Error ? err.message : 'Could not switch source';
    } finally {
      switching = false;
    }
  }
</script>

<div class="page">
  {#if loading}
    <p class="hint">Loading...</p>
  {:else if error}
    <p class="hint">{error}</p>
  {:else if !book}
    <p class="hint">Book not found.</p>
  {:else}
    <div class="book-header">
      {#if book.coverUrl}
        <img src={`/api/books/${encodeURIComponent(book.id)}/cover`} alt={book.name} class="cover" />
      {/if}
      <div class="info">
        <h2>{book.name}</h2>
        {#if book.author}<p class="author">by {book.author}</p>{/if}
        {#if book.kind}<p class="kind">{book.kind}</p>{/if}
        {#if book.lastChapter}<p class="last">{book.lastChapter}</p>{/if}
      </div>
    </div>
    {#if book.intro}
      <p class="intro">{book.intro}</p>
    {/if}
    <div class="source-section">
      <BookSourceSwitcher
        currentSource={book.origin || book.sourceUrl}
        sources={book.alternateSources || []}
        {switching}
        message={switchMessage}
        onselect={switchSource}
      />
    </div>
    {#if continueIndex !== null}
      <button class="continue" onclick={() => go(`read?id=${bookId}&chapter=${continueIndex}`)}>
        {book.durChapterIndex > 0 || book.durChapterPos > 0 ? 'Continue reading' : 'Start reading'}
      </button>
    {/if}

    <h3 class="ch-title">Chapters ({chapters.filter(chapter => !chapter.isVolume).length})</h3>
    <div class="chapter-list">
      {#each chapters as ch}
        {#if ch.isVolume}
          <h4 class="volume-title">{ch.title}</h4>
        {:else}
          <button
            class="chapter-item"
            class:current={ch.index === book.durChapterIndex}
            onclick={() => go(`read?id=${bookId}&chapter=${ch.index}`)}
          >
            {ch.title}
          </button>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .page { padding: 1rem; }
  .book-header { display: flex; gap: 1rem; margin-bottom: 1rem; }
  .cover { width: 96px; height: 128px; object-fit: cover; border-radius: 6px; flex-shrink: 0; }
  .info { flex: 1; }
  .info h2 { font-size: 1.2rem; margin-bottom: 0.3rem; }
  .author { font-size: 0.9rem; color: #888; }
  .kind { font-size: 0.8rem; color: var(--accent); }
  .last { font-size: 0.8rem; color: #999; margin-top: 0.25rem; }
  .intro { font-size: 0.85rem; color: #666; line-height: 1.5; margin-bottom: 1rem; }
  .source-section { margin-bottom: 1rem; }
  .continue {
    width: 100%; margin-bottom: 1rem; padding: 0.65rem; border: 1px solid var(--accent);
    border-radius: 8px; background: var(--accent); color: white; cursor: pointer;
  }
  .ch-title { font-size: 1rem; margin-bottom: 0.5rem; }
  .chapter-list { display: flex; flex-direction: column; gap: 0.25rem; }
  .volume-title { margin: 0.65rem 0 0.2rem; font-size: 0.85rem; color: #666; }
  .chapter-item.current { border-color: var(--accent); }
  .chapter-item {
    text-align: left; padding: 0.6rem 0.8rem; background: var(--card-bg);
    border: 1px solid var(--border); border-radius: 8px;
    font-size: 0.9rem; cursor: pointer;
  }
  .chapter-item:hover { border-color: var(--accent); }
  .hint { color: #999; text-align: center; padding: 2rem; }
</style>
