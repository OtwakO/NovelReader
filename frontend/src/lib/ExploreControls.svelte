<script lang="ts">
  import type { ExploreEntry } from '../api/client';

  let {
    entries,
    disabled,
    onupdate,
  }: {
    entries: ExploreEntry[];
    disabled: boolean;
    onupdate: (entry: ExploreEntry, value: string | null) => void;
  } = $props();

  function submitText(event: SubmitEvent, entry: ExploreEntry) {
    event.preventDefault();
    const input = (event.currentTarget as HTMLFormElement | null)?.querySelector('input');
    if (input) onupdate(entry, input.value);
  }
</script>

{#if entries.length > 0}
  <section class="controls" aria-label="Explore controls">
    {#each entries as entry (entry.id)}
      {#if entry.type === 'text'}
        <form class="control text-control" onsubmit={(event) => submitText(event, entry)}>
          <label for={`control-${entry.id}`}>{entry.title}</label>
          <div class="text-row">
            <input id={`control-${entry.id}`} value={entry.value || ''} disabled={disabled} />
            <button type="submit" disabled={disabled}>Apply</button>
          </div>
        </form>
      {:else if entry.type === 'button'}
        <div class="control action-control">
          <span>Action</span>
          <button type="button" disabled={disabled} onclick={() => onupdate(entry, null)}>{entry.title}</button>
        </div>
      {:else if entry.type === 'select'}
        <label class="control" for={`control-${entry.id}`}>
          <span>{entry.title}</span>
          <select id={`control-${entry.id}`} value={entry.value || ''} disabled={disabled}
            onchange={(event) => onupdate(entry, event.currentTarget.value)}>
            {#each entry.options || [] as option}
              <option value={option}>{option}</option>
            {/each}
          </select>
        </label>
      {:else if entry.type === 'toggle'}
        <fieldset class="control toggle-control" disabled={disabled}>
          <legend>{entry.title}</legend>
          <div class="toggle-options">
            {#each entry.options || [] as option}
              <label>
                <input type="radio" name={`control-${entry.id}`} value={option}
                  checked={entry.value === option}
                  onchange={() => onupdate(entry, option)} />
                <span>{option}</span>
              </label>
            {/each}
          </div>
        </fieldset>
      {/if}
    {/each}
  </section>
{/if}

<style>
  .controls {
    display: grid; grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: 0.6rem; margin-bottom: 0.8rem;
  }
  .control {
    min-width: 0; padding: 0.65rem; background: var(--card-bg);
    border: 1px solid var(--border); border-radius: 10px;
  }
  .control > span, .control > label, legend {
    display: block; margin-bottom: 0.35rem; color: #666;
    font-size: 0.78rem; font-weight: 600;
  }
  fieldset { margin: 0; }
  legend { padding: 0; }
  input, select, button {
    min-height: 2.5rem; border: 1px solid var(--border); border-radius: 8px;
    background: white; color: var(--fg); font: inherit;
  }
  input, select { width: 100%; padding: 0.45rem 0.55rem; }
  button { padding: 0.45rem 0.75rem; cursor: pointer; }
  .text-row { display: flex; gap: 0.4rem; }
  .text-row input { min-width: 0; }
  .action-control button { width: 100%; }
  .toggle-options { display: flex; gap: 0.35rem; flex-wrap: wrap; }
  .toggle-options label { position: relative; }
  .toggle-options input { position: absolute; opacity: 0; pointer-events: none; }
  .toggle-options span {
    display: block; min-height: 2.5rem; padding: 0.5rem 0.7rem;
    border: 1px solid var(--border); border-radius: 8px; cursor: pointer;
  }
  .toggle-options input:checked + span { background: var(--accent); border-color: var(--accent); color: white; }
  input:focus-visible, select:focus-visible, button:focus-visible,
  .toggle-options input:focus-visible + span { outline: 2px solid var(--accent); outline-offset: 2px; }
  :disabled { cursor: not-allowed; opacity: 0.6; }
</style>
