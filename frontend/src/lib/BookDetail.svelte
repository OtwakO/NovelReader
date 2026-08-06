<script lang="ts">
  import { getBook, getChapters, switchBookSource, type Book, type Chapter } from '../api/client';
  import { resolveChapterIndex } from './readingProgress.js';
  import { setProgressVersion, waitForProgressWrites } from './progressWriter';

  let { bookId, go }: { bookId: string; go: (path: string) => void } = $props();

  let book = $state<Book | null>(null);
  let chapters = $state<Chapter[]>([]);
  let loading = $state(true);
  let error = $state('');
  let selectedSource = $state(''), switching = $state(false), switchMessage = $state('');
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
      selectedSource = '';
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

  async function switchSource() {
    const alternate = book?.alternateSources?.[Number(selectedSource)];
    if (!alternate || switching) return;
    switching = true;
    switchMessage = '';
    try {
      await waitForProgressWrites(bookId);
      const result = await switchBookSource(bookId, alternate.sourceUrl, alternate.bookUrl);
      book = result.book;
      setProgressVersion(bookId, result.book.stateVersion);
      chapters = await getChapters(bookId);
      selectedSource = '';
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
    {#if book.alternateSources?.length}
      <section class="sources" aria-labelledby="source-heading">
        <h3 id="source-heading">Reading source</h3>
        <p class="current-source">Current: {book.origin || book.sourceUrl}</p>
        <div class="source-controls">
          <select bind:value={selectedSource} aria-label="Alternate source">
            <option value="">Choose an alternate source</option>
            {#each book.alternateSources as source, index}
              <option value={String(index)}>{source.sourceName || source.sourceUrl}</option>
            {/each}
          </select>
          <button disabled={!selectedSource || switching} onclick={switchSource}>
            {switching ? 'Validating…' : 'Switch'}
          </button>
        </div>
        {#if switchMessage}<p class="source-message" aria-live="polite">{switchMessage}</p>{/if}
      </section>
    {/if}
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
  .sources { margin-bottom: 1rem; padding: 0.8rem; border: 1px solid var(--border); border-radius: 8px; }
  .sources h3 { font-size: 0.9rem; }
  .current-source, .source-message { margin-top: 0.35rem; font-size: 0.8rem; color: #777; overflow-wrap: anywhere; }
  .source-controls { display: flex; gap: 0.5rem; margin-top: 0.6rem; }
  .source-controls select { min-width: 0; flex: 1; padding: 0.5rem; border: 1px solid var(--border); border-radius: 6px; }
  .source-controls button { padding: 0.5rem 0.8rem; border: 0; border-radius: 6px; background: var(--accent); color: white; cursor: pointer; }
  .source-controls button:disabled { opacity: 0.55; cursor: default; }
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
