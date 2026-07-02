<script lang="ts">
  import { listSources, importSources, deleteSource, type BookSource } from '../api/client';

  let { go }: { go: (path: string) => void } = $props();

  let sources = $state<BookSource[]>([]);
  let loading = $state(true);
  let error = $state('');

  $effect(() => {
    load();
  });

  async function load() {
    loading = true;
    try {
      sources = await listSources();
    } catch (e: unknown) {
      error = (e as Error).message;
    }
    loading = false;
  }

  let importing = $state(false);
  async function handleImport(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    importing = true;
    try {
      const text = await file.text();
      const res = await importSources(text);
      alert(`Imported ${res.imported} sources`);
      input.value = '';
      await load();
    } catch (e: unknown) {
      alert('Import failed: ' + (e as Error).message);
    }
    importing = false;
  }

  async function handleDelete(url: string) {
    if (!confirm('Delete this source?')) return;
    await deleteSource(url);
    await load();
  }

  async function toggleSource(s: BookSource) {
    // ponytail: toggle via re-import with updated enabled field
    s.enabled = !s.enabled;
    // Re-import as single source
    await importSources(JSON.stringify(s));
    await load();
  }
</script>

<div class="page">
  <div class="toolbar">
    <h2>Book Sources</h2>
    <label class="import-btn">
      {importing ? 'Importing...' : 'Import JSON'}
      <input type="file" accept=".json" onchange={handleImport} hidden />
    </label>
  </div>

  {#if loading}
    <p class="hint">Loading...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if sources.length === 0}
    <div class="empty">
      <p>No book sources installed.</p>
      <p class="hint">Import a legado-compatible BookSource JSON file to get started.</p>
    </div>
  {:else}
    <div class="source-list">
      {#each sources as s (s.bookSourceUrl)}
        <div class="source-card" class:disabled={!s.enabled}>
          <div class="src-info">
            <strong>{s.bookSourceName}</strong>
            {#if s.bookSourceGroup}
              <span class="group">{s.bookSourceGroup}</span>
            {/if}
            <span class="url">{s.bookSourceUrl}</span>
          </div>
          <div class="src-actions">
            <button class="toggle" onclick={() => toggleSource(s)}>
              {s.enabled ? '✓' : '✗'}
            </button>
            <button class="delete" onclick={() => handleDelete(s.bookSourceUrl)}>🗑</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <div class="footer-hint">
    <p>Tip: Download BookSource JSON from legado source repositories and import them here.</p>
  </div>
</div>

<style>
  .page { padding: 1rem; }
  .toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
  .toolbar h2 { font-size: 1.1rem; }
  .import-btn {
    background: var(--accent); color: white; padding: 0.4rem 0.8rem;
    border-radius: 8px; font-size: 0.85rem; cursor: pointer;
  }
  .source-list { display: flex; flex-direction: column; gap: 0.5rem; }
  .source-card {
    display: flex; justify-content: space-between; align-items: center;
    padding: 0.75rem; background: var(--card-bg); border-radius: 10px;
    border: 1px solid var(--border);
  }
  .source-card.disabled { opacity: 0.5; }
  .src-info { display: flex; flex-direction: column; gap: 0.15rem; flex: 1; min-width: 0; }
  .src-info strong { font-size: 0.95rem; }
  .group { font-size: 0.75rem; color: var(--accent); }
  .url { font-size: 0.75rem; color: #999; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .src-actions { display: flex; gap: 0.25rem; }
  .toggle, .delete {
    background: none; border: 1px solid var(--border); border-radius: 6px;
    padding: 0.3rem 0.5rem; cursor: pointer; font-size: 0.85rem;
  }
  .toggle:hover, .delete:hover { background: var(--bg); }
  .empty { text-align: center; padding: 3rem 1rem; color: #999; }
  .hint { color: #999; font-size: 0.85rem; text-align: center; padding: 1rem; }
  .error { color: #e74c3c; }
  .footer-hint { margin-top: 1.5rem; font-size: 0.8rem; color: #aaa; text-align: center; }
</style>
