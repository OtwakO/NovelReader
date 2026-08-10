<script lang="ts">
  import type { AltSource } from '../api/client';

  let {
    bookName,
    failedSource,
    error,
    sources,
    trying,
    ontry,
    onclose,
  }: {
    bookName: string;
    failedSource: string;
    error: string;
    sources: AltSource[];
    trying: string | null;
    ontry: (source: AltSource) => void;
    onclose: () => void;
  } = $props();
</script>

<section class="source-recovery" aria-labelledby="source-recovery-title" aria-live="polite">
  <div class="recovery-copy">
    <h3 id="source-recovery-title">Choose another source for {bookName}</h3>
    <p><strong>{failedSource}</strong> could not open the book: {error}</p>
    <p class="hint">The book is already on your shelf. Try another source; each choice is checked before the reader opens.</p>
  </div>
  <button class="dismiss" type="button" onclick={onclose} aria-label="Close source choices">×</button>

  {#if sources.length > 0}
    <div class="source-options">
      {#each sources as source (source.sourceUrl + source.bookUrl)}
        <button type="button" disabled={trying !== null} onclick={() => ontry(source)}>
          <span>{source.sourceName || 'Unnamed source'}</span>
          <small>{trying === source.bookUrl ? 'Checking chapters…' : 'Try source'}</small>
        </button>
      {/each}
    </div>
  {:else}
    <p class="no-options">No alternate sources were returned yet. Search more sources, then add the book again.</p>
  {/if}
</section>

<style>
  .source-recovery {
    position: relative;
    margin-block: 0 0.9rem;
    padding: 1rem;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--card-bg);
  }
  .recovery-copy { padding-inline-end: 2.25rem; }
  h3 { margin: 0 0 0.35rem; font-size: 1rem; }
  p { margin: 0.2rem 0; overflow-wrap: anywhere; }
  .hint, .no-options { color: #666; font-size: 0.85rem; }
  .dismiss {
    position: absolute; inset-block-start: 0.55rem; inset-inline-end: 0.55rem;
    width: 2rem; height: 2rem; border: 0; border-radius: 50%;
    background: transparent; color: inherit; font-size: 1.25rem; cursor: pointer;
  }
  .source-options { display: grid; gap: 0.45rem; margin-block-start: 0.85rem; }
  .source-options button {
    display: flex; align-items: center; justify-content: space-between; gap: 1rem;
    width: 100%; padding: 0.7rem 0.8rem; border: 1px solid var(--border);
    border-radius: 9px; background: transparent; color: inherit; text-align: start; cursor: pointer;
  }
  .source-options button:hover:not(:disabled) { border-color: var(--accent); }
  .source-options button:disabled { opacity: 0.6; cursor: wait; }
  .source-options span { min-width: 0; overflow-wrap: anywhere; }
  .source-options small { flex-shrink: 0; color: var(--accent); font-weight: 600; }
  button:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
