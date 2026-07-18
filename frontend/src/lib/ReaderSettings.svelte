<script lang="ts">
  let {
    show = $bindable(), fontSize = $bindable(), lineHeight = $bindable(),
    fontWeight = $bindable(), bgColor = $bindable(), textColor = $bindable(),
    fontFamily = $bindable(), fonts,
  }: {
    show: boolean; fontSize: number; lineHeight: number; fontWeight: number;
    bgColor: string; textColor: string; fontFamily: string;
    fonts: { id: string; name: string; url: string }[];
  } = $props();

  function selectFont(event: Event) {
    const value = (event.target as HTMLSelectElement).value;
    fontFamily = value === 'system' ? "'Georgia', 'Noto Serif SC', serif" : `url('${value}')`;
  }
</script>

{#if show}
  <dialog open class="settings-overlay" aria-label="Typography settings"
    onclick={(event) => { if (event.target === event.currentTarget) show = false; }}
    onkeydown={(event) => { if (event.key === 'Escape') show = false; }}>
    <div class="settings-panel">
      <h3>Typography</h3>

      <label>
        Font Size
        <input type="range" min="12" max="32" bind:value={fontSize} />
        <span>{fontSize}px</span>
      </label>

      <label>
        Line Height
        <input type="range" min="1.2" max="2.5" step="0.1" bind:value={lineHeight} />
        <span>{lineHeight.toFixed(1)}</span>
      </label>

      <label>
        Font Weight
        <select bind:value={fontWeight}>
          <option value={300}>Light (300)</option>
          <option value={400}>Normal (400)</option>
          <option value={500}>Medium (500)</option>
          <option value={700}>Bold (700)</option>
        </select>
      </label>

      <label>
        Font Family
        <select onchange={selectFont}>
          <option value="system">System Default</option>
          {#each fonts as font}
            <option value={font.url}>{font.name}</option>
          {/each}
        </select>
      </label>

      <label>Background <input type="color" bind:value={bgColor} /></label>
      <label>Text Color <input type="color" bind:value={textColor} /></label>

      <h3 class="presets-title">Presets</h3>
      <div class="presets">
        <button onclick={() => { bgColor = '#f5f0eb'; textColor = '#3a3a3a'; }}>Sepia</button>
        <button onclick={() => { bgColor = '#ffffff'; textColor = '#333'; }}>Light</button>
        <button onclick={() => { bgColor = '#1a1a2e'; textColor = '#e0e0e0'; }}>Dark</button>
        <button onclick={() => { bgColor = '#2d2d2d'; textColor = '#c9c9c9'; }}>Grey</button>
      </div>
    </div>
  </dialog>
{/if}

<style>
  .settings-overlay {
    position: fixed; inset: 0; width: 100%; max-width: none; height: 100%;
    margin: 0; padding: 0; border: 0; background: rgba(0,0,0,0.3); z-index: 100;
    display: flex; align-items: flex-end;
  }
  .settings-panel {
    width: 100%; max-height: 70vh; overflow-y: auto; background: var(--card-bg);
    color: var(--fg); border-radius: 16px 16px 0 0; padding: 1.5rem 1.2rem;
  }
  h3 { font-size: 1rem; margin-bottom: 0.75rem; }
  label {
    display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.75rem;
    font-size: 0.85rem; flex-wrap: wrap;
  }
  label input[type="range"] { flex: 1; min-width: 100px; }
  label select {
    flex: 1; padding: 0.3rem; border: 1px solid var(--border);
    border-radius: 6px; background: white;
  }
  label input[type="color"] {
    width: 2.5rem; height: 2rem; border: none; border-radius: 4px; cursor: pointer;
  }
  .presets-title { margin-top: 1rem; }
  .presets { display: flex; gap: 0.5rem; flex-wrap: wrap; }
  .presets button {
    padding: 0.3rem 0.7rem; border: 1px solid var(--border); border-radius: 8px;
    background: var(--bg); cursor: pointer; font-size: 0.8rem;
  }
  .presets button:hover { border-color: var(--accent); }
</style>
