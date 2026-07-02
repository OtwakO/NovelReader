<script lang="ts">
  import { searchBooks, addBook, type SearchResult, type BookSource } from '../api/client';

  let { go }: { go: (path: string) => void } = $props();

  let query = $state('');
  let results = $state<SearchResult[]>([]);
  let searching = $state(false);
  let error = $state('');

  async function handleSearch(e: Event) {
    e.preventDefault();
    if (!query.trim()) return;
    searching = true;
    error = '';
    results = [];
    try {
      results = await searchBooks(query.trim());
    } catch (e: unknown) {
      error = (e as Error).message;
    }
    searching = false;
  }

  let adding = $state<string | null>(null);
  async function addToShelf(r: SearchResult) {
    adding = r.bookUrl;
    try {
      // Fetch book info first
      // ponytail: for now, just add directly with available info
      const book = {
        id: crypto.randomUUID(),
        name: r.name,
        author: r.author,
        coverUrl: r.coverUrl,
        intro: r.intro,
        kind: r.kind,
        sourceUrl: r.sourceUrl,
        bookUrl: r.bookUrl,
      };
      await addBook(book);
      go(`book?id=${book.id}`);
    } catch (e: unknown) {
      alert('Failed: ' + (e as Error).message);
    }
    adding = null;
  }
</script>

<div class="page">
  <form class="search-bar" onsubmit={handleSearch}>
    <input
      type="search"
      bind:value={query}
      placeholder="Search books..."
      autofocus
      // svelte-ignore a11y_autofocus
    />
    <button type="submit" disabled={searching}>
      {searching ? '...' : 'Search'}
    </button>
  </form>

  {#if error}
    <p class="error">{error}</p>
  {/if}

  {#if results.length > 0}
    <div class="results">
      {#each results as r}
        <div class="result-card">
          {#if r.coverUrl}
            <img src={r.coverUrl} alt={r.name} class="cover" loading="lazy" />
          {/if}
          <div class="info">
            <strong>{r.name}</strong>
            <span class="author">{r.author}</span>
            {#if r.kind}
              <span class="kind">{r.kind}</span>
            {/if}
            {#if r.lastChapter}
              <span class="last">{r.lastChapter}</span>
            {/if}
            <span class="source">{r.sourceName}</span>
          </div>
          <button class="add-btn" onclick={() => addToShelf(r)} disabled={adding === r.bookUrl}>
            +
          </button>
        </div>
      {/each}
    </div>
  {:else if searching}
    <p class="hint">Searching...</p>
  {/if}
</div>

<style>
  .page { padding: 1rem; }
  .search-bar { display: flex; gap: 0.5rem; margin-bottom: 1rem; }
  .search-bar input {
    flex: 1; padding: 0.6rem 0.8rem; border: 1px solid var(--border);
    border-radius: 10px; font-size: 1rem; background: var(--card-bg);
  }
  .search-bar button {
    background: var(--accent); color: white; border: none;
    padding: 0.6rem 1rem; border-radius: 10px; font-size: 0.95rem; cursor: pointer;
  }
  .results { display: flex; flex-direction: column; gap: 0.5rem; }
  .result-card {
    display: flex; gap: 0.75rem; align-items: center;
    padding: 0.75rem; background: var(--card-bg); border-radius: 10px;
    border: 1px solid var(--border);
  }
  .cover { width: 48px; height: 64px; object-fit: cover; border-radius: 4px; flex-shrink: 0; }
  .info { flex: 1; display: flex; flex-direction: column; gap: 0.15rem; min-width: 0; }
  .info strong { font-size: 0.95rem; }
  .author { font-size: 0.8rem; color: #888; }
  .kind { font-size: 0.75rem; color: var(--accent); }
  .last { font-size: 0.75rem; color: #999; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .source { font-size: 0.7rem; color: #aaa; }
  .add-btn {
    background: var(--accent); color: white; border: none;
    width: 2rem; height: 2rem; border-radius: 50%; font-size: 1.2rem;
    cursor: pointer; flex-shrink: 0;
  }
  .add-btn:disabled { opacity: 0.5; }
  .hint { color: #999; font-size: 0.85rem; text-align: center; padding: 2rem; }
  .error { color: #e74c3c; margin-bottom: 0.5rem; }
</style>
