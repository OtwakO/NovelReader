<script lang="ts">
  import { listFonts, uploadFont, deleteFont, getFontUrl, type Font } from '../api/client';

  let fonts = $state<Font[]>([]);
  let uploading = $state(false);

  $effect(() => {
    load();
  });

  async function load() {
    fonts = await listFonts();
  }

  async function handleUpload(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    // Validate it's a font file
    if (!file.name.match(/\.(ttf|otf|woff|woff2)$/i)) {
      alert('Please select a font file (.ttf, .otf, .woff, .woff2)');
      return;
    }

    uploading = true;
    try {
      await uploadFont(file, file.name.replace(/\.[^.]+$/, ''));
      input.value = '';
      await load();
    } catch (e: unknown) {
      alert('Upload failed: ' + (e as Error).message);
    }
    uploading = false;
  }

  async function handleDelete(id: string) {
    if (!confirm('Delete this font?')) return;
    await deleteFont(id);
    await load();
  }
</script>

<div class="page">
  <h2>Settings</h2>

  <section>
    <h3>Fonts</h3>
    <p class="hint">Upload custom fonts to use in the reader.</p>

    <label class="upload-btn">
      {uploading ? 'Uploading...' : 'Upload Font'}
      <input type="file" accept=".ttf,.otf,.woff,.woff2" onchange={handleUpload} hidden />
    </label>

    {#if fonts.length > 0}
      <div class="font-list">
        {#each fonts as f}
          <div class="font-item">
            <span class="font-name">{f.name}</span>
            <span class="font-size">{(f.fileSize / 1024).toFixed(1)} KB</span>
            <div class="font-preview" style="font-family: url('{getFontUrl(f.id)}');">
              Aa 你好
            </div>
            <button class="delete-btn" onclick={() => handleDelete(f.id)}>🗑</button>
          </div>
        {/each}
      </div>
    {:else}
      <p class="hint">No custom fonts uploaded.</p>
    {/if}
  </section>

  <section>
    <h3>Reader Defaults</h3>
    <p class="hint">Typography settings are available in the reader. Defaults will be persisted here in a future update.</p>
  </section>
</div>

<style>
  .page { padding: 1rem; }
  .page h2 { font-size: 1.1rem; margin-bottom: 1rem; }

  section { margin-bottom: 1.5rem; }
  section h3 { font-size: 1rem; margin-bottom: 0.5rem; }

  .upload-btn {
    display: inline-block;
    background: var(--accent); color: white; padding: 0.5rem 1rem;
    border-radius: 8px; font-size: 0.85rem; cursor: pointer;
    margin-bottom: 1rem;
  }

  .font-list { display: flex; flex-direction: column; gap: 0.5rem; }
  .font-item {
    display: flex; align-items: center; gap: 0.75rem;
    padding: 0.6rem 0.8rem; background: var(--card-bg);
    border: 1px solid var(--border); border-radius: 8px;
  }
  .font-name { font-size: 0.9rem; flex: 1; }
  .font-size { font-size: 0.75rem; color: #999; }
  .font-preview {
    font-size: 1rem;
    padding: 0.2rem 0.5rem;
    border: 1px solid var(--border);
    border-radius: 4px;
    min-width: 80px;
    text-align: center;
  }
  .delete-btn {
    background: none; border: 1px solid var(--border);
    border-radius: 6px; padding: 0.3rem 0.5rem; cursor: pointer;
  }

  .hint { font-size: 0.8rem; color: #999; margin-bottom: 0.5rem; }
</style>
