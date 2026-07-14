<script lang="ts">
  let {
    checked, eligible, resultCount, searching, effectiveConcurrency,
    sourceFailures, error, storageWarning, restartRequired, retryRequired,
    hasMore, batchSize, onrestart, onretry, onmore,
  }: {
    checked: number; eligible: number; resultCount: number; searching: boolean;
    effectiveConcurrency: number; sourceFailures: number; error: string;
    storageWarning: string; restartRequired: boolean; retryRequired: boolean;
    hasMore: boolean; batchSize: number; onrestart: () => void;
    onretry: () => void; onmore: () => void;
  } = $props();

  let moreCount = $derived(Math.min(batchSize, Math.max(0, eligible - checked)));
</script>

<section class="search-status" aria-live="polite">
  {#if eligible > 0}
    <progress value={checked} max={eligible}></progress>
  {/if}
  <div class="status-row">
    <span>
      {#if eligible > 0}{checked} of {eligible} sources checked{:else}{checked} sources checked{/if}
      · {resultCount} book{#if resultCount !== 1}s{/if} found
    </span>
    {#if searching}<span>Concurrency {effectiveConcurrency}</span>{/if}
  </div>
  {#if sourceFailures > 0}<p class="hint">{sourceFailures} source failures</p>{/if}
  {#if error}<p class="error">{error}</p>{/if}
  {#if storageWarning}<p class="error">{storageWarning}</p>{/if}

  {#if !searching}
    <div class="search-actions">
      {#if restartRequired}
        <button type="button" onclick={onrestart}>Restart search</button>
      {:else if retryRequired}
        <button type="button" onclick={onretry}>Retry search batch</button>
      {:else if hasMore}
        <button type="button" onclick={onmore}>Search {moreCount} more</button>
      {/if}
    </div>
  {/if}
</section>

<style>
  .search-status {
    padding: 0.75rem; margin-bottom: 0.75rem;
    background: var(--card-bg); border: 1px solid var(--border); border-radius: 10px;
  }
  progress { display: block; width: 100%; height: 0.5rem; margin-bottom: 0.5rem; accent-color: var(--accent); }
  .status-row { display: flex; justify-content: space-between; gap: 0.75rem; font-size: 0.8rem; color: #666; }
  .search-actions { margin-top: 0.65rem; }
  .search-actions button {
    background: var(--accent); color: white; border: none;
    padding: 0.6rem 1rem; border-radius: 10px; font-size: 0.9rem; cursor: pointer;
  }
  .search-actions button:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .hint { font-size: 0.75rem; color: #888; margin-top: 0.35rem; }
  .error { color: #b42318; font-size: 0.8rem; margin-top: 0.4rem; }
  @media (max-width: 420px) {
    .status-row { flex-direction: column; gap: 0.2rem; }
  }
</style>
