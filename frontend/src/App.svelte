<script lang="ts">
  import { onMount } from 'svelte';
  import Bookshelf from './lib/Bookshelf.svelte';
  import SourceList from './lib/SourceList.svelte';
  import SearchPage from './lib/SearchPage.svelte';
  import BookDetail from './lib/BookDetail.svelte';
  import Reader from './lib/Reader.svelte';
  import Settings from './lib/Settings.svelte';

  let route = $state('shelf');
  let params = $state<Record<string, string>>({});

  function navigate(hash: string) {
    const [path, ...rest] = hash.replace(/^#\//, '').split('?');
    route = path || 'shelf';
    params = {};
    if (rest.length) {
      rest.join('?').split('&').forEach(p => {
        const [k, v] = p.split('=');
        if (k) params[decodeURIComponent(k)] = decodeURIComponent(v || '');
      });
    }
  }

  onMount(() => {
    const onHash = () => navigate(window.location.hash);
    window.addEventListener('hashchange', onHash);
    if (window.location.hash) onHash();
    else window.location.hash = '#/shelf';
    return () => window.removeEventListener('hashchange', onHash);
  });

  function go(path: string) {
    window.location.hash = '#/' + path;
  }

  function goBack() {
    if (route === 'read' || route === 'book') history.back();
    else go('shelf');
  }
</script>

<div class="app">
  <header class="app-header">
    <button class="back" onclick={goBack}>←</button>
    <h1 onclick={() => go('shelf')} style="cursor:pointer">NovelReader</h1>
    <nav>
      <button class="nav-btn" class:active={route==='shelf'} onclick={() => go('shelf')} aria-label="Shelf">📖</button>
      <button class="nav-btn" class:active={route==='search'} onclick={() => go('search')} aria-label="Search">🔍</button>
      <button class="nav-btn" class:active={route==='sources'} onclick={() => go('sources')} aria-label="Sources">📚</button>
      <button class="nav-btn" class:active={route==='settings'} onclick={() => go('settings')} aria-label="Settings">⚙️</button>
    </nav>
  </header>

  <main class="app-main">
    {#if route === 'shelf'}
      <Bookshelf {go} />
    {:else if route === 'search'}
      <SearchPage {go} />
    {:else if route === 'sources'}
      <SourceList {go} />
    {:else if route === 'book'}
      <BookDetail bookId={params.id || ''} {go} />
    {:else if route === 'read'}
      <Reader bookId={params.id || ''} chapterIdx={parseInt(params.chapter || '0')} {go} />
    {:else if route === 'settings'}
      <Settings />
    {:else}
      <Bookshelf {go} />
    {/if}
  </main>
</div>

<style>
  :root {
    --bg: #f5f0eb;
    --fg: #3a3a3a;
    --accent: #8b5cf6;
    --card-bg: #ffffff;
    --border: #e5e0db;
    --nav-bg: #ffffff;
  }

  :global(*) { box-sizing: border-box; margin: 0; padding: 0; }
  :global(body) { font-family: system-ui, -apple-system, sans-serif; background: var(--bg); color: var(--fg); }
  :global(img) { max-width: 100%; }

  .app {
    min-height: 100dvh; display: flex; flex-direction: column;
  }

  .app-header {
    display: flex; align-items: center; gap: 0.5rem;
    padding: 0.5rem 1rem; background: var(--nav-bg);
    border-bottom: 1px solid var(--border);
    position: sticky; top: 0; z-index: 10;
  }

  .app-header h1 {
    font-size: 1.1rem; font-weight: 600; flex: 1;
  }

  .back {
    background: none; border: none; font-size: 1.2rem;
    cursor: pointer; padding: 0.25rem;
  }

  nav { display: flex; gap: 0.25rem; }

  .nav-btn {
    background: none; border: none; font-size: 1.2rem;
    cursor: pointer; padding: 0.4rem 0.5rem; border-radius: 8px;
  }
  .nav-btn:hover { background: var(--bg); }
  .nav-btn.active { background: var(--accent); color: white; }

  .app-main { flex: 1; overflow-y: auto; }
</style>
