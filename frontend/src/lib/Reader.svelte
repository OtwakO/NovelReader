<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import { getBook, getChapterContent, getChapters, listFonts, getFontUrl, type Chapter, type ChapterContent } from '../api/client';
  import { adjacentChapterIndex, normalizedScroll, resolveChapterIndex, scrollTopForProgress } from './readingProgress.js';
  import { queueProgressWrite, waitForProgressWrites } from './progressWriter';
  import ReaderSettings from './ReaderSettings.svelte';

  let { bookId, chapterIdx, go }: { bookId: string; chapterIdx?: number; go: (path: string) => void } = $props();
  let fontSize = $state(18), lineHeight = $state(1.8), fontWeight = $state(400);
  let bgColor = $state('#f5f0eb'), textColor = $state('#3a3a3a');
  let fontFamily = $state("'Georgia', 'Noto Serif SC', serif"), showSettings = $state(false);
  let content = $state<ChapterContent | null>(null), chapters = $state<Chapter[]>([]);
  let currentIdx = $state(0), loading = $state(true), error = $state(''), progressError = $state('');
  let fonts = $state<{ id: string; name: string; url: string }[]>([]);
  let root: HTMLElement, scrollHost: HTMLElement | null = null, loadedRoute = '';
  let generation = 0, restoring = false, destroyed = false, lastPosition = 0;
  let progressTimer: ReturnType<typeof setTimeout> | undefined;
  let previousIdx = $derived(adjacentChapterIndex(chapters, currentIdx, -1));
  let nextIdx = $derived(adjacentChapterIndex(chapters, currentIdx, 1));

  $effect(() => {
    const route = `${bookId}:${chapterIdx === undefined ? 'resume' : chapterIdx}`;
    if (route !== loadedRoute) {
      loadedRoute = route;
      void load(route);
    }
  });

  onMount(() => {
    scrollHost = root.closest('.app-main') as HTMLElement | null;
    scrollHost?.addEventListener('scroll', scheduleProgress, { passive: true });
    void loadFonts();
  });

  onDestroy(() => {
    destroyed = true;
    generation += 1;
    scrollHost?.removeEventListener('scroll', scheduleProgress);
    if (progressTimer) clearTimeout(progressTimer);
    void persistProgress();
  });

  async function load(route: string) {
    const request = ++generation;
    loading = true; error = ''; content = null;
    try {
      await waitForProgressWrites(bookId);
      if (request !== generation || route !== loadedRoute) return;
      const [book, nextChapters] = await Promise.all([getBook(bookId), getChapters(bookId)]);
      if (request !== generation || route !== loadedRoute) return;
      chapters = nextChapters;
      const index = resolveChapterIndex(nextChapters, chapterIdx, book.durChapterIndex);
      if (index === null) throw new Error('This book has no readable chapters');
      const position = index === book.durChapterIndex ? book.durChapterPos || 0 : 0;
      lastPosition = position;
      const nextContent = await getChapterContent(bookId, index);
      if (request !== generation || route !== loadedRoute) return;
      currentIdx = index;
      content = nextContent;
      loading = false;
      await restoreProgress(position, request);
      if (index !== book.durChapterIndex) await queueProgress(index, position);
    } catch (caught) {
      if (request !== generation) return;
      error = caught instanceof Error ? caught.message : 'Could not load this chapter';
      loading = false;
    }
  }

  async function restoreProgress(position: number, request: number) {
    await tick();
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
    if (request !== generation || !scrollHost) return;
    restoring = true;
    scrollHost.scrollTop = scrollTopForProgress(position, scrollHost.scrollHeight, scrollHost.clientHeight);
    requestAnimationFrame(() => { restoring = false; });
  }

  function scheduleProgress() {
    if (loading || restoring || !content || !scrollHost) return;
    lastPosition = normalizedScroll(scrollHost.scrollTop, scrollHost.scrollHeight, scrollHost.clientHeight);
    if (progressTimer) clearTimeout(progressTimer);
    progressTimer = setTimeout(() => void persistProgress(), 600);
  }

  function persistProgress() {
    if (!content || !bookId) return Promise.resolve();
    return queueProgress(currentIdx, lastPosition);
  }

  async function queueProgress(index: number, position: number) {
    const id = bookId;
    try {
      await queueProgressWrite(id, index, position);
      if (!destroyed && id === bookId) progressError = '';
    } catch {
      if (!destroyed && id === bookId) progressError = 'Progress could not be saved.';
    }
  }

  async function navigateChapter(index: number | null) {
    if (index === null) return;
    if (progressTimer) clearTimeout(progressTimer);
    await persistProgress();
    go(`read?id=${bookId}&chapter=${index}`);
  }

  async function loadFonts() {
    try {
      const list = await listFonts();
      if (!destroyed) fonts = list.map(f => ({ id: f.id, name: f.name, url: getFontUrl(f.id) }));
    } catch { /* optional */ }
  }

</script>

<div bind:this={root} class="reader-container" style="background: {bgColor}; color: {textColor};">
  <ReaderSettings bind:show={showSettings} bind:fontSize bind:lineHeight bind:fontWeight
    bind:bgColor bind:textColor bind:fontFamily {fonts} />

  <!-- Reader content -->
  <div
    class="reader-content"
    style="font-size: {fontSize}px; line-height: {lineHeight}; font-weight: {fontWeight}; font-family: {fontFamily};"
  >
    {#if loading}
      <p class="loading">Loading...</p>
    {:else if error}
      <p class="reader-error" role="alert">{error}</p>
    {:else if content}
      {#each content.paragraphs as p}
        <p class="paragraph">{p}</p>
      {/each}
    {/if}
  </div>

  <!-- Bottom controls -->
  {#if progressError}<p class="progress-error" role="status">{progressError}</p>{/if}
  <div class="reader-footer">
    <button class="ctrl-btn" onclick={() => navigateChapter(previousIdx)} disabled={previousIdx === null}>← Prev</button>

    <button class="ctrl-btn settings-btn" onclick={() => showSettings = !showSettings} aria-label="Typography settings">
      {showSettings ? '✕' : 'Aa'}
    </button>

    <button class="ctrl-btn" onclick={() => navigateChapter(nextIdx)} disabled={nextIdx === null}>Next →</button>
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

  .loading, .reader-error {
    text-align: center;
    padding: 3rem;
  }
  .loading { opacity: 0.5; }
  .reader-error, .progress-error { color: #b42318; }
  .progress-error { margin: 0; padding: 0.35rem 1rem; text-align: center; font-size: 0.8rem; }

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

</style>
