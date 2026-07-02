<script lang="ts">
  import { searchBooksStream, enrichBook, addBook, type SearchResult } from '../api/client';

  let { go }: { go: (path: string) => void } = $props();

  let query = $state('');
  let results = $state<SearchResult[]>([]);
  let searching = $state(false);
  let sourcesDone = $state(0);
  let totalResults = $state(0);
  let error = $state('');
  let es = $state<EventSource | null>(null);

  // Clean up EventSource on unmount (route change, back nav)
  $effect(() => {
    return () => {
      if (es) { es.close(); es = null; }
    };
  });

  function handleSearch(e: Event) {
    e.preventDefault();
    if (!query.trim()) return;

    // Cancel previous search
    if (es) es.close();

    results = [];
    sourcesDone = 0;
    totalResults = 0;
    error = '';
    searching = true;

    const q = query.trim();
    es = searchBooksStream(
      q,
      (source, items) => {
        results = [...results, ...items];
        // Sort by relevance: higher score first, stable so arrival order is tiebreaker
        results.sort((a, b) => (b.score || 0) - (a.score || 0));
        sourcesDone++;
      },
      (source, msg) => {
        sourcesDone++;
      },
      (total, done) => {
        totalResults = total;
        sourcesDone = done;
        searching = false;
        es = null;
      }
    );
  }

  function cancelSearch() {
    if (es) { es.close(); es = null; }
    searching = false;
  }

  let adding = $state<string | null>(null);
  async function addToShelf(r: SearchResult) {
    adding = r.bookUrl;
    try {
      const id = crypto.randomUUID();
      try {
        await enrichBook({
          id, name: r.name, author: r.author || '',
          coverUrl: r.coverUrl || '', intro: r.intro || '',
          sourceUrl: r.sourceUrl, bookUrl: r.bookUrl,
          alternateSources: r.alternateSources,
        });
      } catch {
        await addBook({
          id, name: r.name, author: r.author, coverUrl: r.coverUrl,
          intro: r.intro, kind: r.kind, sourceUrl: r.sourceUrl, bookUrl: r.bookUrl,
        });
      }
      go(`book?id=${id}`);
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
    />
    {#if searching}
      <button type="button" class="cancel-btn" onclick={cancelSearch}>✕</button>
    {:else}
      <button type="submit">Search</button>
    {/if}
  </form>

  {#if searching}
    <div class="search-status">
      <span class="spinner"></span>
      <span>Searching... {results.length} results from {sourcesDone} sources</span>
    </div>
  {:else if error}
    <p class="error">{error}</p>
  {/if}

  {#if results.length > 0}
    <div class="result-count">{results.length} results · {sourcesDone} sources</div>
    <div class="results">
      {#each results as r (r.sourceUrl + r.bookUrl)}
        <div class="result-card">
          {#if r.coverUrl}
            <img src={r.coverUrl} alt={r.name} class="cover" loading="lazy" />
          {/if}
          <div class="info">
            <strong>{r.name}</strong>
            {#if r.author}<span class="author">{r.author}</span>{/if}
            {#if r.kind}<span class="kind">{r.kind}</span>{/if}
            {#if r.lastChapter}<span class="last">{r.lastChapter}</span>{/if}
            <span class="source">{r.sourceName}</span>
            {#if r.alternateSources && r.alternateSources.length > 0}
              <span class="alt-count">+{r.alternateSources.length} more</span>
            {/if}
          </div>
          <button class="add-btn" onclick={() => addToShelf(r)} disabled={adding === r.bookUrl}>
            +
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .page { padding: 1rem; }
  .search-bar { display: flex; gap: 0.5rem; margin-bottom: 0.5rem; }
  .search-bar input {
    flex: 1; padding: 0.6rem 0.8rem; border: 1px solid var(--border);
    border-radius: 10px; font-size: 1rem; background: var(--card-bg);
  }
  .search-bar button, .cancel-btn {
    background: var(--accent); color: white; border: none;
    padding: 0.6rem 1rem; border-radius: 10px; font-size: 0.95rem; cursor: pointer;
  }
  .cancel-btn { background: #e74c3c; }
  .search-bar button:disabled { opacity: 0.5; }

  .search-status {
    display: flex; align-items: center; gap: 0.5rem;
    font-size: 0.85rem; color: #888; margin-bottom: 0.75rem;
  }
  .spinner {
    width: 1rem; height: 1rem; border: 2px solid var(--border);
    border-top-color: var(--accent); border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .result-count { font-size: 0.8rem; color: #999; margin-bottom: 0.5rem; }

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
  .alt-count { font-size: 0.7rem; color: var(--accent); }
  .add-btn {
    background: var(--accent); color: white; border: none;
    width: 2rem; height: 2rem; border-radius: 50%; font-size: 1.2rem;
    cursor: pointer; flex-shrink: 0;
  }
  .add-btn:disabled { opacity: 0.5; }
  .error { color: #e74c3c; margin-bottom: 0.5rem; }
</style>
