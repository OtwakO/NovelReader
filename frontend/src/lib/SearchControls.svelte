<script lang="ts">
  import type { SearchIntensity } from './searchPreferences';

  let {
    batchSize = $bindable(),
    intensity = $bindable(),
    advancedConcurrency = $bindable(),
    onchange,
  }: {
    batchSize: number;
    intensity: SearchIntensity;
    advancedConcurrency: number;
    onchange: () => void;
  } = $props();

  function changed() {
    batchSize = Math.min(500, Math.max(1, Math.trunc(batchSize || 1)));
    advancedConcurrency = Math.max(1, Math.trunc(advancedConcurrency || 1));
    onchange();
  }
</script>

<div class="controls" aria-label="Search controls">
  <label>
    <span>Sources per batch</span>
    <input type="number" min="1" max="500" bind:value={batchSize} onchange={changed} />
  </label>
  <label>
    <span>Search intensity</span>
    <select bind:value={intensity} onchange={changed}>
      <option value="gentle">Gentle · 4</option>
      <option value="balanced">Balanced · 8</option>
      <option value="fast">Fast · 16</option>
      <option value="advanced">Advanced</option>
    </select>
  </label>
  {#if intensity === 'advanced'}
    <label>
      <span>Concurrency</span>
      <input type="number" min="1" bind:value={advancedConcurrency} onchange={changed} />
    </label>
  {/if}
</div>

<style>
  .controls {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
    gap: 0.5rem;
    margin-bottom: 0.75rem;
  }
  label { display: flex; flex-direction: column; gap: 0.25rem; }
  label span { font-size: 0.72rem; color: #777; font-weight: 600; }
  input, select {
    width: 100%; min-height: 2.25rem; padding: 0.4rem 0.55rem;
    border: 1px solid var(--border); border-radius: 8px;
    background: var(--card-bg); color: var(--fg); font: inherit;
  }
  input:focus, select:focus { outline: 2px solid var(--accent); outline-offset: 1px; }
</style>
