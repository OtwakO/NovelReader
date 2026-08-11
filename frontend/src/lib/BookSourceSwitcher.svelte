<script lang="ts">
  import type { AltSource } from '../api/client';

  let {
    currentSource,
    sources,
    switching,
    message,
    onselect,
  }: {
    currentSource: string;
    sources: AltSource[];
    switching: boolean;
    message: string;
    onselect: (source: AltSource) => void;
  } = $props();

  let selected = $state('');

  function submit() {
    const source = sources[Number(selected)];
    if (!source || switching) return;
    selected = '';
    onselect(source);
  }
</script>

<section class="sources" aria-labelledby="source-heading">
  <h3 id="source-heading">Reading source</h3>
  <p class="current-source">Current: {currentSource}</p>
  {#if sources.length > 0}
    <div class="source-controls">
      <select bind:value={selected} aria-label="Alternate source" disabled={switching}>
        <option value="">Choose an alternate source</option>
        {#each sources as source, index}
          <option value={String(index)}>{source.sourceName || source.sourceUrl}</option>
        {/each}
      </select>
      <button type="button" disabled={!selected || switching} onclick={submit}>
        {switching ? 'Switching…' : 'Switch'}
      </button>
    </div>
  {:else}
    <p class="no-alternates">No alternate sources are stored for this book.</p>
  {/if}
  {#if message}<p class="source-message" role="status">{message}</p>{/if}
</section>

<style>
  .sources { padding: 0.8rem; border: 1px solid var(--border); border-radius: 8px; background: var(--card-bg); color: var(--fg); }
  h3 { margin: 0; font-size: 0.9rem; }
  .current-source, .source-message, .no-alternates {
    margin: 0.35rem 0 0; font-size: 0.8rem; color: #777; overflow-wrap: anywhere;
  }
  .source-controls { display: flex; gap: 0.5rem; margin-top: 0.6rem; }
  select { min-width: 0; flex: 1; padding: 0.5rem; border: 1px solid var(--border); border-radius: 6px; background: var(--card-bg); color: inherit; }
  button { padding: 0.5rem 0.8rem; border: 0; border-radius: 6px; background: var(--accent); color: white; cursor: pointer; }
  button:disabled, select:disabled { opacity: 0.55; cursor: default; }
  button:focus-visible, select:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  @media (max-width: 480px) {
    .source-controls { align-items: stretch; flex-direction: column; }
  }
</style>
