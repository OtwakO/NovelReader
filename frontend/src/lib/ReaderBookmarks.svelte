<script lang="ts">
  import { addBookmark, deleteBookmark, listBookmarks, type Bookmark } from '../api/client';

  let {
    show = $bindable(), bookId, sourceUrl, chapterIndex, capture, open,
  }: {
    show: boolean; bookId: string; sourceUrl: string; chapterIndex: number;
    capture: () => Promise<{ position: number; stateVersion: number }>;
    open: (chapterIndex: number, position: number) => void;
  } = $props();
  let bookmarks = $state<Bookmark[]>([]), note = $state(''), busy = $state(false), error = $state('');
  let loadedSource = '';

  $effect(() => {
    const source = `${bookId}:${sourceUrl}`;
    if (sourceUrl && source !== loadedSource) {
      loadedSource = source;
      void load();
    }
  });

  async function load() {
    try {
      bookmarks = await listBookmarks(bookId);
      error = '';
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Could not load bookmarks';
    }
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (busy) return;
    busy = true;
    error = '';
    try {
      const location = await capture();
      const mark = await addBookmark(bookId, {
        id: crypto.randomUUID(), sourceUrl, stateVersion: location.stateVersion,
        chapterIndex, position: location.position, note,
      });
      bookmarks = [mark, ...bookmarks];
      note = '';
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Could not save bookmark';
    } finally {
      busy = false;
    }
  }

  async function remove(mark: Bookmark) {
    error = '';
    try {
      await deleteBookmark(bookId, mark.id);
      bookmarks = bookmarks.filter((item) => item.id !== mark.id);
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Could not delete bookmark';
    }
  }
</script>

{#if show}
  <dialog open class="bookmark-overlay" aria-label="Bookmarks"
    onclick={(event) => { if (event.target === event.currentTarget) show = false; }}
    onkeydown={(event) => { if (event.key === 'Escape') show = false; }}>
    <section class="bookmark-panel">
      <header><h3>Bookmarks</h3><button aria-label="Close bookmarks" onclick={() => show = false}>✕</button></header>
      <form onsubmit={submit}>
        <textarea maxlength="1000" bind:value={note} placeholder="Optional note" aria-label="Bookmark note"></textarea>
        <button disabled={busy}>Bookmark current position</button>
      </form>
      {#if error}<p class="error" role="status">{error}</p>{/if}
      {#if bookmarks.length === 0}
        <p class="empty">No bookmarks yet.</p>
      {:else}
        <ul>
          {#each bookmarks as bookmark (bookmark.id)}
            <li class:orphaned={bookmark.orphaned}>
              <div>
                <strong>{bookmark.chapterTitle}</strong>
                <span>{Math.round(bookmark.position * 100)}%{bookmark.orphaned ? ' · unavailable in this source' : ''}</span>
                {#if bookmark.note}<p>{bookmark.note}</p>{/if}
              </div>
              <div class="actions">
                <button disabled={bookmark.orphaned} onclick={() => { show = false; open(bookmark.chapterIndex, bookmark.position); }}>Open</button>
                <button aria-label={`Delete bookmark ${bookmark.chapterTitle}`} onclick={() => remove(bookmark)}>Delete</button>
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  </dialog>
{/if}

<style>
  .bookmark-overlay { position: fixed; inset: 0; width: 100%; max-width: none; height: 100%; margin: 0; padding: 0; border: 0; background: rgba(0,0,0,.3); z-index: 100; display: flex; align-items: flex-end; }
  .bookmark-panel { width: 100%; max-height: 75vh; overflow-y: auto; padding: 1rem; border-radius: 16px 16px 0 0; background: var(--card-bg); color: var(--fg); }
  header, .actions { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }
  header button { border: 0; background: none; cursor: pointer; font-size: 1rem; }
  form { display: grid; gap: .5rem; margin: .8rem 0; }
  textarea { min-height: 4rem; resize: vertical; padding: .55rem; border: 1px solid var(--border); border-radius: 6px; }
  form button, .actions button { padding: .45rem .65rem; border: 1px solid var(--border); border-radius: 6px; background: var(--bg); color: inherit; cursor: pointer; }
  button:disabled { opacity: .5; cursor: default; }
  ul { list-style: none; display: grid; gap: .55rem; }
  li { display: flex; justify-content: space-between; gap: .8rem; padding: .65rem; border: 1px solid var(--border); border-radius: 8px; }
  li > div:first-child { min-width: 0; }
  strong, span { display: block; }
  strong, li p { overflow-wrap: anywhere; }
  span { margin-top: .15rem; font-size: .75rem; color: #777; }
  li p { margin-top: .35rem; font-size: .85rem; white-space: pre-wrap; }
  .actions { align-items: flex-start; flex-wrap: wrap; justify-content: flex-end; }
  .orphaned { opacity: .7; }
  .empty, .error { padding: 1rem 0; font-size: .85rem; }
  .error { color: #b42318; }
</style>
