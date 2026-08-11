<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import {
    getBook, getChapterContent, getChapterImageUrl, getChapters, listFonts, getFontUrl,
    switchBookSource, type AltSource, type Book, type Chapter, type ChapterContent,
  } from '../api/client';
  import { adjacentChapterIndex, clampProgress, normalizedScroll, resolveChapterIndex, scrollTopForProgress } from './readingProgress.js';
  import { getProgressVersion, queueProgressWrite, setProgressVersion, waitForProgressWrites } from './progressWriter';
  import BookSourceSwitcher from './BookSourceSwitcher.svelte';
  import ReaderBookmarks from './ReaderBookmarks.svelte';
  import ReaderSettings from './ReaderSettings.svelte';

  let { bookId, chapterIdx, locationPos, go }: { bookId: string; chapterIdx?: number; locationPos?: number; go: (path: string) => void } = $props();
  let fontSize = $state(18), lineHeight = $state(1.8), fontWeight = $state(400);
  let bgColor = $state('#f5f0eb'), textColor = $state('#3a3a3a');
  let fontFamily = $state("'Georgia', 'Noto Serif SC', serif"), showSettings = $state(false);
  let content = $state<ChapterContent | null>(null), chapters = $state<Chapter[]>([]);
  let currentIdx = $state(0), loading = $state(true), error = $state(''), progressError = $state('');
  let showBookmarks = $state(false), showSources = $state(false);
  let book = $state<Book | null>(null), switchingSource = $state(false), sourceMessage = $state('');
  let fonts = $state<{ id: string; name: string; url: string }[]>([]);
  let root: HTMLElement, scrollHost: HTMLElement | null = null, loadedRoute = '';
  let sourceURL = $state('');
  let generation = 0, restoring = false, destroyed = false, lastPosition = 0;
  let progressTimer: ReturnType<typeof setTimeout> | undefined;
  let previousIdx = $derived(adjacentChapterIndex(chapters, currentIdx, -1));
  let nextIdx = $derived(adjacentChapterIndex(chapters, currentIdx, 1));

  $effect(() => {
    const route = `${bookId}:${chapterIdx === undefined ? 'resume' : chapterIdx}:${Number.isFinite(locationPos) ? locationPos : ''}`;
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
      const [nextBook, nextChapters] = await Promise.all([getBook(bookId), getChapters(bookId)]);
      if (request !== generation || route !== loadedRoute) return;
      book = nextBook;
      chapters = nextChapters;
      sourceURL = nextBook.sourceUrl;
      setProgressVersion(bookId, nextBook.stateVersion);
      const index = resolveChapterIndex(nextChapters, chapterIdx, nextBook.durChapterIndex);
      if (index === null) throw new Error('This book has no readable chapters');
      const position = Number.isFinite(locationPos) ? clampProgress(locationPos) : index === nextBook.durChapterIndex ? nextBook.durChapterPos || 0 : 0;
      lastPosition = position;
      const nextContent = await getChapterContent(bookId, index);
      if (request !== generation || route !== loadedRoute) return;
      currentIdx = index;
      content = nextContent;
      loading = false;
      await restoreProgress(position, request);
      if (index !== nextBook.durChapterIndex) await queueProgress(index, position);
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
      await queueProgressWrite(id, sourceURL, index, position);
      if (!destroyed && id === bookId) progressError = '';
    } catch {
      if (!destroyed && id === bookId) progressError = 'Progress could not be saved.';
    }
  }

  async function captureBookmark() {
    if (!content || !scrollHost) throw new Error('Reading position is not ready');
    lastPosition = normalizedScroll(scrollHost.scrollTop, scrollHost.scrollHeight, scrollHost.clientHeight);
    if (progressTimer) clearTimeout(progressTimer);
    await persistProgress();
    const stateVersion = getProgressVersion(bookId);
    if (stateVersion === undefined) throw new Error('Reading state is not ready');
    return { position: lastPosition, stateVersion };
  }

  async function navigateChapter(index: number | null) {
    if (index === null) return;
    if (progressTimer) clearTimeout(progressTimer);
    await persistProgress();
    go(`read?id=${bookId}&chapter=${index}`);
  }

  async function switchSource(source: AltSource) {
    if (!book || switchingSource) return;
    switchingSource = true;
    sourceMessage = '';
    if (progressTimer) clearTimeout(progressTimer);
    try {
      await persistProgress();
      await waitForProgressWrites(bookId);
      const result = await switchBookSource(bookId, source.sourceUrl, source.bookUrl);
      book = result.book;
      sourceURL = result.book.sourceUrl;
      setProgressVersion(bookId, result.book.stateVersion);
      content = null;
      const nextChapters = await getChapters(bookId);
      chapters = nextChapters;
      const mappedIndex = resolveChapterIndex(nextChapters, result.book.durChapterIndex, result.book.durChapterIndex);
      if (mappedIndex === null) throw new Error('This source has no readable chapters');
      currentIdx = mappedIndex;
      lastPosition = result.book.durChapterPos || 0;
      sourceMessage = result.mapping === 'title'
        ? 'Source switched at the matching chapter.'
        : 'Source switched using the nearest chapter index.';
      if (mappedIndex !== chapterIdx) {
        go(`read?id=${bookId}&chapter=${mappedIndex}`);
        return;
      }
      content = await getChapterContent(bookId, mappedIndex);
      error = '';
      await restoreProgress(lastPosition, generation);
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : 'Could not switch source';
      sourceMessage = message;
      if (!content) error = message;
    } finally {
      switchingSource = false;
    }
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
  <ReaderBookmarks bind:show={showBookmarks} {bookId} sourceUrl={sourceURL} chapterIndex={currentIdx}
    capture={captureBookmark} open={(index, position) => go(`read?id=${bookId}&chapter=${index}&position=${position}`)} />

  {#if showSources && book}
    <div class="reader-sources">
      <BookSourceSwitcher
        currentSource={book.origin || book.sourceUrl}
        sources={book.alternateSources || []}
        switching={switchingSource}
        message={sourceMessage}
        onselect={switchSource}
      />
    </div>
  {/if}

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
      {#if content.offlineCopy}
        <p class="offline-copy" role="status">Offline copy — the source is currently unavailable.</p>
      {/if}
      {#if content.blocks.length > 0}
        {#each content.blocks as block}
          {#if block.type === 'text'}
            <p class="paragraph">{block.text}</p>
          {:else}
            <figure class="chapter-image-frame">
              <img
                class="chapter-image"
                src={getChapterImageUrl(bookId, currentIdx, block.index)}
                alt="Illustration from {content.title}"
                loading="lazy"
                decoding="async"
              />
            </figure>
          {/if}
        {/each}
      {:else}
        {#each content.paragraphs as p}
          <p class="paragraph">{p}</p>
        {/each}
      {/if}
    {/if}
  </div>

  <!-- Bottom controls -->
  {#if progressError}<p class="progress-error" role="status">{progressError}</p>{/if}
  <div class="reader-footer">
    <button class="ctrl-btn" onclick={() => navigateChapter(previousIdx)} disabled={previousIdx === null}>← Prev</button>

    <button class="ctrl-btn settings-btn" onclick={() => showSettings = !showSettings} aria-label="Typography settings">
      {showSettings ? '✕' : 'Aa'}
    </button>

    <button class="ctrl-btn settings-btn" onclick={() => showBookmarks = !showBookmarks} aria-label="Bookmarks">🔖</button>

    <button class="ctrl-btn" class:active={showSources} onclick={() => showSources = !showSources} aria-label="Change reading source">Source</button>

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

  .reader-sources {
    position: sticky;
    top: 0;
    z-index: 5;
    width: min(680px, calc(100% - 2rem));
    margin: 0.75rem auto 0;
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

  .chapter-image-frame {
    margin: 1.2rem 0;
  }

  .chapter-image {
    display: block;
    width: 100%;
    height: auto;
    max-height: 85dvh;
    object-fit: contain;
    margin-inline: auto;
  }

  .loading, .reader-error {
    text-align: center;
    padding: 3rem;
  }
  .loading { opacity: 0.5; }
  .reader-error, .progress-error { color: #b42318; }
  .offline-copy { margin-bottom: 1rem; padding: 0.6rem; border: 1px solid #b7791f; border-radius: 6px; color: #7a4f12; font-size: 0.85rem; }
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

  .ctrl-btn.active { border-color: var(--accent); color: var(--accent); }

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
