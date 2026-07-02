<script lang="ts">
  import { listBooks, type Book } from '../api/client';

  let { go }: { go: (path: string) => void } = $props();

  let books = $state<Book[]>([]);
  let loading = $state(true);

  $effect(() => { load(); });

  async function load() {
    loading = true;
    try {
      books = await listBooks();
    } catch (_) { /* ignore */ }
    loading = false;
  }
</script>

<div class="shelf">
  <div class="shelf-header">
    <h2>My Bookshelf</h2>
    <button class="refresh" onclick={load}>↻</button>
  </div>

  {#if loading}
    <p class="hint">Loading...</p>
  {:else if books.length === 0}
    <div class="empty">
      <p>Your bookshelf is empty.</p>
      <p class="hint">Search for books and add them to your shelf to start reading.</p>
      <button class="search-btn" onclick={() => go('search')}>🔍 Search Books</button>
    </div>
  {:else}
    <div class="book-list">
      {#each books as b (b.id)}
        <button class="book-card" onclick={() => go(`book?id=${b.id}`)}>
          {#if b.coverUrl}
            <img src={b.coverUrl} alt={b.name} class="cover" loading="lazy" onerror={(e) => (e.target as HTMLImageElement).style.display='none'} />
          {:else}
            <div class="cover-placeholder">{b.name[0]}</div>
          {/if}
          <div class="info">
            <strong class="title">{b.name}</strong>
            <span class="author">{b.author || 'Unknown'}</span>
            {#if b.lastChapter}
              <span class="last-ch">📖 {b.lastChapter}</span>
            {/if}
            <div class="progress">
              <span class="prog-text">
                Ch.{b.durChapterIndex + 1} / {b.totalChapterNum || '?'}
              </span>
              {#if b.totalChapterNum > 0}
                <div class="prog-bar">
                  <div class="prog-fill" style="width: {Math.min(100, (b.durChapterIndex / b.totalChapterNum) * 100)}%"></div>
                </div>
              {/if}
            </div>
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .shelf { padding: 1rem; }

  .shelf-header {
    display: flex; justify-content: space-between; align-items: center;
    margin-bottom: 1rem;
  }
  .shelf-header h2 { font-size: 1.1rem; }
  .refresh {
    background: none; border: 1px solid var(--border); border-radius: 8px;
    padding: 0.3rem 0.6rem; cursor: pointer; font-size: 1rem;
  }

  .book-list { display: flex; flex-direction: column; gap: 0.75rem; }

  .book-card {
    display: flex; gap: 0.75rem; padding: 0.75rem;
    background: var(--card-bg); border: 1px solid var(--border);
    border-radius: 12px; cursor: pointer; text-align: left;
    width: 100%; transition: border-color 0.15s;
  }
  .book-card:hover { border-color: var(--accent); }
  .book-card:active { transform: scale(0.99); }

  .cover {
    width: 64px; height: 88px; object-fit: cover; border-radius: 6px;
    flex-shrink: 0; background: var(--bg);
  }
  .cover-placeholder {
    width: 64px; height: 88px; border-radius: 6px; flex-shrink: 0;
    background: var(--accent); color: white; display: flex;
    align-items: center; justify-content: center;
    font-size: 1.5rem; font-weight: 600;
  }

  .info {
    flex: 1; display: flex; flex-direction: column; gap: 0.2rem;
    min-width: 0; overflow: hidden;
  }
  .title { font-size: 0.95rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .author { font-size: 0.8rem; color: #888; }
  .last-ch { font-size: 0.75rem; color: #999; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  .progress { margin-top: 0.3rem; }
  .prog-text { font-size: 0.7rem; color: #aaa; }
  .prog-bar {
    height: 3px; background: var(--border); border-radius: 2px;
    margin-top: 0.15rem; overflow: hidden;
  }
  .prog-fill { height: 100%; background: var(--accent); border-radius: 2px; transition: width 0.3s; }

  .empty { text-align: center; padding: 3rem 1rem; color: #999; }
  .hint { color: #999; font-size: 0.85rem; text-align: center; padding: 1rem; }
  .search-btn {
    margin-top: 1rem; background: var(--accent); color: white;
    border: none; padding: 0.6rem 1.2rem; border-radius: 10px;
    font-size: 0.95rem; cursor: pointer;
  }
</style>
