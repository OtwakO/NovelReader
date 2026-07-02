<script lang="ts">
  import { listBooks, getChapters, type Book, type Chapter } from '../api/client';

  let { bookId, go }: { bookId: string; go: (path: string) => void } = $props();

  let book = $state<Book | null>(null);
  let chapters = $state<Chapter[]>([]);
  let loading = $state(true);

  $effect(() => {
    load();
  });

  async function load() {
    loading = true;
    try {
      const books = await listBooks();
      book = books.find(b => b.id === bookId) || null;
      if (book) {
        chapters = await getChapters(bookId);
      }
    } catch (_) { /* ignore */ }
    loading = false;
  }
</script>

<div class="page">
  {#if loading}
    <p class="hint">Loading...</p>
  {:else if !book}
    <p class="hint">Book not found.</p>
  {:else}
    <div class="book-header">
      {#if book.coverUrl}
        <img src={book.coverUrl} alt={book.name} class="cover" />
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

    <h3 class="ch-title">Chapters ({chapters.length})</h3>
    <div class="chapter-list">
      {#each chapters as ch}
        <button
          class="chapter-item"
          onclick={() => go(`read?id=${bookId}&chapter=${ch.index}`)}
        >
          {ch.title}
        </button>
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
  .ch-title { font-size: 1rem; margin-bottom: 0.5rem; }
  .chapter-list { display: flex; flex-direction: column; gap: 0.25rem; }
  .chapter-item {
    text-align: left; padding: 0.6rem 0.8rem; background: var(--card-bg);
    border: 1px solid var(--border); border-radius: 8px;
    font-size: 0.9rem; cursor: pointer;
  }
  .chapter-item:hover { border-color: var(--accent); }
  .hint { color: #999; text-align: center; padding: 2rem; }
</style>
