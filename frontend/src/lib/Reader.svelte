<script lang="ts">
  import { getChapterContent, saveProgress, getChapters, listFonts, getFontUrl, type Chapter, type ChapterContent } from '../api/client';

  let { bookId, chapterIdx, go }: { bookId: string; chapterIdx: number; go: (path: string) => void } = $props();

  // Typography settings
  let fontSize = $state(18);
  let lineHeight = $state(1.8);
  let fontWeight = $state(400);
  let bgColor = $state('#f5f0eb');
  let textColor = $state('#3a3a3a');
  let fontFamily = $state("'Georgia', 'Noto Serif SC', serif");
  let showSettings = $state(false);

  // Content
  let content = $state<ChapterContent | null>(null);
  let chapters = $state<Chapter[]>([]);
  let currentIdx = $state(chapterIdx);
  let loading = $state(true);

  // Fonts
  let fonts = $state<{ id: string; name: string; url: string }[]>([]);

  $effect(() => {
    load();
    loadFonts();
  });

  async function load() {
    loading = true;
    try {
      chapters = await getChapters(bookId);
      await loadChapter(currentIdx);
    } catch (_) { /* ignore */ }
    loading = false;
  }

  async function loadFonts() {
    try {
      const list = await listFonts();
      fonts = list.map(f => ({ id: f.id, name: f.name, url: getFontUrl(f.id) }));
    } catch (_) { /* ignore */ }
  }

  async function loadChapter(idx: number) {
    currentIdx = idx;
    content = null;
    try {
      content = await getChapterContent(bookId, idx);
      // Save progress
      saveProgress(bookId, idx, 0).catch(() => {});
    } catch (_) { /* ignore */ }
  }

  function prevChapter() {
    if (currentIdx > 0) loadChapter(currentIdx - 1);
  }

  function nextChapter() {
    if (currentIdx < chapters.length - 1) loadChapter(currentIdx + 1);
  }

  function selectFont(e: Event) {
    const select = e.target as HTMLSelectElement;
    const val = select.value;
    if (val === 'system') {
      fontFamily = "'Georgia', 'Noto Serif SC', serif";
    } else {
      fontFamily = `url('${val}')`;
    }
  }
</script>

<div class="reader-container" style="background: {bgColor}; color: {textColor};">
  <!-- Settings panel -->
  {#if showSettings}
    <div class="settings-overlay" onclick={() => showSettings = false}>
      <div class="settings-panel" onclick={(e) => e.stopPropagation()}>
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
            {#each fonts as f}
              <option value={f.url}>{f.name}</option>
            {/each}
          </select>
        </label>

        <label>
          Background
          <input type="color" bind:value={bgColor} />
        </label>

        <label>
          Text Color
          <input type="color" bind:value={textColor} />
        </label>

        <h3 style="margin-top: 1rem;">Presets</h3>
        <div class="presets">
          <button onclick={() => { bgColor = '#f5f0eb'; textColor = '#3a3a3a'; }}>Sepia</button>
          <button onclick={() => { bgColor = '#ffffff'; textColor = '#333'; }}>Light</button>
          <button onclick={() => { bgColor = '#1a1a2e'; textColor = '#e0e0e0'; }}>Dark</button>
          <button onclick={() => { bgColor = '#2d2d2d'; textColor = '#c9c9c9'; }}>Grey</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Reader content -->
  <div
    class="reader-content"
    style="font-size: {fontSize}px; line-height: {lineHeight}; font-weight: {fontWeight}; font-family: {fontFamily};"
  >
    {#if loading || !content}
      <p class="loading">Loading...</p>
    {:else}
      {#each content.paragraphs as p, i}
        <p class="paragraph">{p}</p>
      {/each}
    {/if}
  </div>

  <!-- Bottom controls -->
  <div class="reader-footer">
    <button class="ctrl-btn" onclick={prevChapter} disabled={currentIdx <= 0}>← Prev</button>

    <button class="ctrl-btn settings-btn" onclick={() => showSettings = !showSettings}>
      {showSettings ? '✕' : 'Aa'}
    </button>

    <button class="ctrl-btn" onclick={nextChapter} disabled={currentIdx >= chapters.length - 1}>Next →</button>
  </div>
</div>

<style>
  .reader-container {
    min-height: 100dvh;
    display: flex;
    flex-direction: column;
    position: relative;
    transition: background 0.2s, color 0.2s;
  }

  .reader-content {
    flex: 1;
    padding: 1.5rem 1.2rem;
    max-width: 680px;
    margin: 0 auto;
    width: 100%;
    overflow-y: auto;
  }

  .paragraph {
    margin-bottom: 0.4em;
    word-break: break-word;
    letter-spacing: 0.02em;
  }

  .loading {
    text-align: center;
    padding: 3rem;
    opacity: 0.5;
  }

  .reader-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.6rem 1rem;
    border-top: 1px solid rgba(128,128,128,0.15);
    background: inherit;
    position: sticky;
    bottom: 0;
  }

  .ctrl-btn {
    background: none;
    border: 1px solid rgba(128,128,128,0.3);
    border-radius: 8px;
    padding: 0.5rem 1rem;
    font-size: 0.85rem;
    cursor: pointer;
    color: inherit;
  }
  .ctrl-btn:disabled { opacity: 0.3; cursor: default; }
  .ctrl-btn:hover:not(:disabled) { border-color: var(--accent); }

  .settings-btn {
    width: 2.5rem; height: 2.5rem;
    display: flex; align-items: center; justify-content: center;
    font-size: 1rem;
  }

  /* Settings overlay */
  .settings-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.3);
    z-index: 100;
    display: flex;
    align-items: flex-end;
  }

  .settings-panel {
    width: 100%;
    max-height: 70vh;
    overflow-y: auto;
    background: var(--card-bg);
    color: var(--fg);
    border-radius: 16px 16px 0 0;
    padding: 1.5rem 1.2rem;
  }

  .settings-panel h3 {
    font-size: 1rem;
    margin-bottom: 0.75rem;
  }

  .settings-panel label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.75rem;
    font-size: 0.85rem;
    flex-wrap: wrap;
  }

  .settings-panel label input[type="range"] {
    flex: 1;
    min-width: 100px;
  }

  .settings-panel label select {
    flex: 1;
    padding: 0.3rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: white;
  }

  .settings-panel label input[type="color"] {
    width: 2.5rem;
    height: 2rem;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .presets {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .presets button {
    padding: 0.3rem 0.7rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg);
    cursor: pointer;
    font-size: 0.8rem;
  }
  .presets button:hover { border-color: var(--accent); }
</style>
