<script lang="ts">
  import type { SearchResult } from '../api/client';

  let {
    results,
    adding,
    onadd,
  }: {
    results: SearchResult[];
    adding: string | null;
    onadd: (result: SearchResult) => void;
  } = $props();

  let multipleSourceCount = $derived(results.filter((result) => result.alternateSources?.length).length);
</script>

<div class="result-count">
  {results.length} book{#if results.length !== 1}s{/if}
  {#if multipleSourceCount > 0} · {multipleSourceCount} with multiple sources{/if}
</div>
<div class="results">
  {#each results as result (result.sourceUrl + result.bookUrl)}
    <div class="result-card">
      {#if result.coverUrl}
        <img src={result.coverUrl} alt={result.name} class="cover" loading="lazy" />
      {/if}
      <div class="info">
        <strong>{result.name}</strong>
        {#if result.author}<span class="author">{result.author}</span>{/if}
        {#if result.kind}<span class="kind">{result.kind}</span>{/if}
        {#if result.lastChapter}<span class="last">{result.lastChapter}</span>{/if}
        <div class="source-row">
          <span class="source">{result.sourceName}</span>
          {#if result.alternateSources?.length}
            <span class="alt-count">+{result.alternateSources.length} sources</span>
          {/if}
        </div>
      </div>
      <button class="add-btn" onclick={() => onadd(result)} disabled={adding === result.bookUrl} aria-label={`Add ${result.name} to shelf`}>
        +
      </button>
    </div>
  {/each}
</div>

<style>
  .result-count { font-size: 0.8rem; color: #777; margin-bottom: 0.5rem; }
  .results { display: flex; flex-direction: column; gap: 0.5rem; }
  .result-card {
    display: flex; gap: 0.75rem; align-items: center;
    padding: 0.75rem; background: var(--card-bg); border-radius: 10px;
    border: 1px solid var(--border);
  }
  .cover { width: 48px; height: 64px; object-fit: cover; border-radius: 4px; flex-shrink: 0; }
  .info { flex: 1; display: flex; flex-direction: column; gap: 0.15rem; min-width: 0; }
  .info strong { font-size: 0.95rem; }
  .author { font-size: 0.8rem; color: #777; }
  .kind { font-size: 0.75rem; color: var(--accent); }
  .last { font-size: 0.75rem; color: #888; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .source-row { display: flex; gap: 0.4rem; align-items: center; flex-wrap: wrap; }
  .source { font-size: 0.7rem; color: #888; }
  .alt-count { font-size: 0.7rem; color: var(--accent); font-weight: 600; }
  .add-btn {
    background: var(--accent); color: white; border: none;
    width: 2rem; height: 2rem; border-radius: 50%; font-size: 1.2rem;
    cursor: pointer; flex-shrink: 0;
  }
  .add-btn:disabled { opacity: 0.5; }
  .add-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
