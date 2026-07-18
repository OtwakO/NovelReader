<script lang="ts">
  import { onMount } from 'svelte';
  import Bookshelf from './lib/Bookshelf.svelte';
  import SourceList from './lib/SourceList.svelte';
  import SearchPage from './lib/SearchPage.svelte';
  import ExplorePage from './lib/ExplorePage.svelte';
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
    <button class="back" onclick={goBack} aria-label="Go back">←</button>
    <h1><button class="brand" onclick={() => go('shelf')}>NovelReader</button></h1>
    <nav aria-label="Primary">
      <button class="nav-btn" class:active={route==='shelf'} aria-current={route==='shelf' ? 'page' : undefined} onclick={() => go('shelf')} aria-label="Shelf">📖</button>
      <button class="nav-btn" class:active={route==='explore'} aria-current={route==='explore' ? 'page' : undefined} onclick={() => go('explore')} aria-label="Explore">🧭</button>
      <button class="nav-btn" class:active={route==='search'} aria-current={route==='search' ? 'page' : undefined} onclick={() => go('search')} aria-label="Search">🔍</button>
      <button class="nav-btn" class:active={route==='sources'} aria-current={route==='sources' ? 'page' : undefined} onclick={() => go('sources')} aria-label="Sources">📚</button>
      <button class="nav-btn" class:active={route==='settings'} aria-current={route==='settings' ? 'page' : undefined} onclick={() => go('settings')} aria-label="Settings">⚙️</button>
    </nav>
  </header>

  <main class="app-main">
    {#if route === 'shelf'}
      <Bookshelf {go} />
    {:else if route === 'explore'}
      <ExplorePage {go} />
    {:else if route === 'search'}
      <SearchPage {go} />
    {:else if route === 'sources'}
      <SourceList {go} />
    {:else if route === 'book'}
      <BookDetail bookId={params.id || ''} {go} />
    {:else if route === 'read'}
      <Reader bookId={params.id || ''} chapterIdx={params.chapter === undefined ? undefined : parseInt(params.chapter)} {go} />
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
    height: 100dvh; overflow: hidden; display: flex; flex-direction: column;
  }

  .app-header {
    display: flex; align-items: center; gap: 0.5rem;
    padding: 0.5rem 1rem; background: var(--nav-bg);
    border-bottom: 1px solid var(--border);
    position: sticky; top: 0; z-index: 10;
  }

  .app-header h1 { font-size: 1.1rem; font-weight: 600; flex: 1; }
  .brand, .back {
    background: none; border: none; color: inherit; cursor: pointer;
  }
  .brand { font: inherit; font-weight: inherit; }
  .back { font-size: 1.2rem; padding: 0.25rem; }

  nav { display: flex; gap: 0.25rem; }

  .nav-btn {
    background: none; border: none; font-size: 1.2rem;
    cursor: pointer; padding: 0.4rem 0.5rem; border-radius: 8px;
  }
  .nav-btn:hover { background: var(--bg); }
  .nav-btn.active { background: var(--accent); color: white; }
  .brand:focus-visible, .back:focus-visible, .nav-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }

  .app-main { flex: 1; overflow-y: auto; }

  @media (max-width: 360px) {
    .app-header { padding-inline: 0.5rem; }
    .app-header h1 { display: none; }
    nav { margin-left: auto; }
  }
</style>
