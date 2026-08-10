<script lang="ts">
  import { onMount } from 'svelte';
  import Bookshelf from './Bookshelf.svelte';
  import SourceList from './SourceList.svelte';
  import SearchPage from './SearchPage.svelte';
  import ExplorePage from './ExplorePage.svelte';
  import BookDetail from './BookDetail.svelte';
  import Reader from './Reader.svelte';
  import Settings from './Settings.svelte';
  import AccountPage from './AccountPage.svelte';
  import type { AuthAccount } from '../api/client';

  let { account, onLogout, onPasswordChanged }: { account: AuthAccount; onLogout: () => void; onPasswordChanged: () => void } = $props();
  let route = $state('shelf');
  let params = $state<Record<string, string>>({});

  function navigate(hash: string) {
    const [path, ...rest] = hash.replace(/^#\//, '').split('?');
    route = path && !['login', 'register', 'recovery'].includes(path) ? path : 'shelf';
    params = {};
    if (rest.length) rest.join('?').split('&').forEach(p => {
      const [k, v] = p.split('=');
      if (k) params[decodeURIComponent(k)] = decodeURIComponent(v || '');
    });
  }
  onMount(() => {
    const onHash = () => navigate(window.location.hash);
    window.addEventListener('hashchange', onHash);
    if (window.location.hash) onHash(); else window.location.hash = '#/shelf';
    return () => window.removeEventListener('hashchange', onHash);
  });
  function go(path: string) { window.location.hash = '#/' + path; }
  function goBack() { if (route === 'read' || route === 'book') history.back(); else go('shelf'); }
</script>

<div class="app">
  <header class="app-header">
    <button class="back" onclick={goBack} aria-label="Go back">←</button>
    <h1><button class="brand" onclick={() => go('shelf')}>NovelReader</button></h1>
    <nav aria-label="Primary">
      <button class="nav-btn" class:active={route==='shelf'} onclick={() => go('shelf')} aria-label="Shelf">📖</button>
      <button class="nav-btn" class:active={route==='explore'} onclick={() => go('explore')} aria-label="Explore">🧭</button>
      <button class="nav-btn" class:active={route==='search'} onclick={() => go('search')} aria-label="Search">🔍</button>
      <button class="nav-btn" class:active={route==='sources'} onclick={() => go('sources')} aria-label="Sources">📚</button>
      <button class="nav-btn" class:active={route==='settings'} onclick={() => go('settings')} aria-label="Settings">⚙️</button>
    </nav>
    <button class="account" class:active={route==='account'} onclick={() => go('account')} aria-label={`Account settings for ${account.username}`}>{account.username}</button>
  </header>
  <main class="app-main">
    {#if route === 'shelf'}<Bookshelf {go} />
    {:else if route === 'explore'}<ExplorePage {go} />
    {:else if route === 'search'}<SearchPage {go} />
    {:else if route === 'sources'}<SourceList {go} />
    {:else if route === 'book'}<BookDetail bookId={params.id || ''} {go} />
    {:else if route === 'read'}<Reader bookId={params.id || ''} chapterIdx={params.chapter === undefined ? undefined : parseInt(params.chapter)} locationPos={params.position === undefined ? undefined : parseFloat(params.position)} {go} />
    {:else if route === 'settings'}<Settings />
    {:else if route === 'account'}<AccountPage {account} {onLogout} {onPasswordChanged} />
    {:else}<Bookshelf {go} />{/if}
  </main>
</div>

<style>
  :root { --bg:#f5f0eb; --fg:#3a3a3a; --accent:#8b5cf6; --card-bg:#fff; --border:#e5e0db; --nav-bg:#fff; }
  :global(*) { box-sizing:border-box; margin:0; padding:0; }
  :global(body) { font-family:system-ui,-apple-system,sans-serif; background:var(--bg); color:var(--fg); }
  :global(img) { max-width:100%; }
  .app { height:100dvh; overflow:hidden; display:flex; flex-direction:column; }
  .app-header { display:flex; align-items:center; gap:.5rem; padding:.5rem 1rem; background:var(--nav-bg); border-bottom:1px solid var(--border); position:sticky; top:0; z-index:10; }
  .app-header h1 { font-size:1.1rem; font-weight:600; flex:1; }
  .brand,.back { background:none; border:none; color:inherit; cursor:pointer; }
  .brand { font:inherit; font-weight:inherit; }
  .back { font-size:1.2rem; padding:.25rem; }
  nav { display:flex; gap:.25rem; }
  .nav-btn { background:none; border:none; font-size:1.2rem; cursor:pointer; padding:.4rem .5rem; border-radius:8px; }
  .nav-btn:hover { background:var(--bg); }
  .nav-btn.active { background:var(--accent); color:white; }
  .account { max-width:9rem; overflow:hidden; text-overflow:ellipsis; padding:.45rem .65rem; border:1px solid var(--border); border-radius:.5rem; background:white; color:inherit; cursor:pointer; }
  button:focus-visible { outline:2px solid var(--accent); outline-offset:2px; }
  .app-main { flex:1; overflow-y:auto; }
  @media (max-width:520px) { .app-header { padding-inline:.5rem; } .app-header h1,.account { display:none; } nav { margin-left:auto; } }
</style>
