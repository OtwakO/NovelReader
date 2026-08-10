<script lang="ts">
  import { login, type AuthAccount } from '../api/client';

  let { registrationEnabled, onLogin, onRegister, onRecovery }: { registrationEnabled?: boolean; onLogin?: (account: AuthAccount) => void; onRegister?: () => void; onRecovery?: () => void } = $props();
  let username = $state('');
  let password = $state('');
  let error = $state('');
  let submitting = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    submitting = true;
    try {
      const account = await login(username, password);
      password = '';
      onLogin?.(account);
    } catch (cause) {
      password = '';
      error = cause instanceof Error ? cause.message : 'Sign in failed.';
    } finally {
      submitting = false;
    }
  }
</script>

<main class="login-shell">
  <section class="login-card" aria-labelledby="login-title">
    <p class="eyebrow">NovelReader</p>
    <h1 id="login-title">Welcome back</h1>
    <p class="intro">Sign in to open your private bookshelf, sources, progress, and bookmarks.</p>
    <form onsubmit={submit}>
      <label for="login-username">Username</label>
      <input id="login-username" bind:value={username} autocomplete="username" required disabled={submitting} />
      <label for="login-password">Password</label>
      <input id="login-password" bind:value={password} type="password" autocomplete="current-password" required disabled={submitting} />
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button type="submit" disabled={submitting}>{submitting ? 'Signing in…' : 'Sign in'}</button>
    </form>
    <div class="secondary-actions">
      {#if registrationEnabled}<button type="button" onclick={() => onRegister?.()}>Create account</button>{/if}
      <button type="button" onclick={() => onRecovery?.()}>Administrator recovery</button>
    </div>
  </section>
</main>

<style>
  .login-shell { min-height: 100dvh; display: grid; place-items: center; padding: 1.5rem; background: #f5f0eb; color: #3a3a3a; font-family: system-ui, sans-serif; }
  .login-card { width: min(100%, 28rem); padding: 2rem; border: 1px solid #e5e0db; border-radius: 1rem; background: white; box-shadow: 0 1rem 2.5rem rgb(58 58 58 / 9%); }
  .eyebrow { margin: 0 0 .4rem; color: #7448df; font-size: .75rem; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
  h1 { margin: 0; font-size: 2rem; }
  .intro { margin: .7rem 0 1.5rem; color: #68625d; line-height: 1.5; }
  form { display: grid; gap: .65rem; }
  label { font-size: .9rem; font-weight: 700; }
  input { min-height: 2.75rem; padding: .65rem .75rem; border: 1px solid #cfc8c1; border-radius: .55rem; font: inherit; }
  input:focus-visible, button:focus-visible { outline: 2px solid #8b5cf6; outline-offset: 2px; }
  button { min-height: 2.75rem; border: 0; border-radius: .55rem; background: #8b5cf6; color: white; font: inherit; font-weight: 750; cursor: pointer; }
  button:disabled { opacity: .65; cursor: wait; }
  .error { margin: .3rem 0; padding: .7rem; border-radius: .5rem; background: #fff2f2; color: #852c2c; }
  .secondary-actions { display:flex; justify-content:center; gap:1rem; flex-wrap:wrap; margin-top:1rem; }
  .secondary-actions button { min-height:auto; padding:.25rem; background:transparent; color:#5d35c7; font-weight:650; }
</style>
