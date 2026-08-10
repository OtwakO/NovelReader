<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError, getCurrentAccount, getRecoveryStatus, getRegistrationPolicy, getSetupStatus, logout, onAuthenticationLoss, type AuthAccount } from './api/client';
  import AuthenticatedApp from './lib/AuthenticatedApp.svelte';
  import LoginPage from './lib/LoginPage.svelte';
  import SetupPage from './lib/SetupPage.svelte';
  import RecoveryPage from './lib/RecoveryPage.svelte';
  import RegistrationPage from './lib/RegistrationPage.svelte';

  type Gate = 'loading' | 'setup' | 'setup-unavailable' | 'login' | 'registration' | 'logout-failed' | 'recovery' | 'recovery-unavailable' | 'authenticated' | 'error';
  let gate: Gate = $state('loading');
  let account: AuthAccount | null = $state(null);
  let message = $state('');
  let requestedHash = '';
  let registrationEnabled = $state(false);
  let registrationInviteRequired = $state(false);

  onMount(() => {
    const stopListening = onAuthenticationLoss(() => {
      if (gate === 'authenticated') {
        requestedHash = window.location.hash || '#/shelf';
        account = null;
        gate = 'login';
        message = 'Your session ended. Sign in again to continue.';
        window.location.hash = '#/login';
      }
    });
    void bootstrap();
    return stopListening;
  });

  async function bootstrap() {
    gate = 'loading';
    message = '';
    requestedHash = window.location.hash && !['#/login', '#/register', '#/recovery'].includes(window.location.hash) ? window.location.hash : '#/shelf';
    try {
      const setup = await getSetupStatus();
      if (setup.status !== 'closed') {
        gate = setup.available ? 'setup' : 'setup-unavailable';
        return;
      }
      const registration = await getRegistrationPolicy();
      registrationEnabled = registration.enabled;
      registrationInviteRequired = registration.inviteRequired;
      try {
        account = await getCurrentAccount();
        gate = 'authenticated';
        window.location.hash = requestedHash;
        return;
      } catch (cause) {
        if (!(cause instanceof ApiError) || cause.status !== 401) throw cause;
      }
      if (window.location.hash === '#/register' && registrationEnabled) {
        gate = 'registration';
        return;
      }
      if (window.location.hash === '#/recovery') {
        await showRecovery();
        return;
      }
      gate = 'login';
    } catch (cause) {
      message = cause instanceof Error ? cause.message : 'NovelReader could not start.';
      gate = 'error';
    }
  }

  function signedIn(next: AuthAccount) {
    message = '';
    account = next;
    gate = 'authenticated';
    window.location.hash = requestedHash || '#/shelf';
  }

  function showRegistration() {
    if (!registrationEnabled) return;
    gate = 'registration';
    window.location.hash = '#/register';
  }

  function showLogin() {
    gate = 'login';
    window.location.hash = '#/login';
  }

  async function showRecovery() {
    try {
      const status = await getRecoveryStatus();
      gate = status.available ? 'recovery' : 'recovery-unavailable';
      window.location.hash = '#/recovery';
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 404) gate = 'recovery-unavailable';
      else {
        message = cause instanceof Error ? cause.message : 'Recovery status could not be loaded.';
        gate = 'error';
      }
    }
  }

  async function signOut() {
    requestedHash = '#/shelf';
    account = null;
    gate = 'loading';
    try {
      await logout();
      message = '';
      gate = 'login';
      window.location.hash = '#/login';
    } catch (cause) {
      message = cause instanceof Error ? cause.message : 'The server could not revoke this session.';
      gate = 'logout-failed';
    }
  }
</script>

{#if gate === 'loading'}
  <main class="gate-message" aria-busy="true"><p>Opening NovelReader…</p></main>
{:else if gate === 'setup'}
  <SetupPage onComplete={signedIn} />
{:else if gate === 'setup-unavailable'}
  <main class="gate-message"><section><h1>Setup requires server configuration</h1><p>Set <code>ADMIN_BOOTSTRAP_TOKEN</code>, restart NovelReader, and reload this page.</p></section></main>
{:else if gate === 'login'}
  {#if message}<p class="session-message" role="status">{message}</p>{/if}
  <LoginPage {registrationEnabled} onLogin={signedIn} onRegister={showRegistration} onRecovery={showRecovery} />
{:else if gate === 'registration'}
  <RegistrationPage inviteRequired={registrationInviteRequired} onComplete={signedIn} onCancel={showLogin} />
{:else if gate === 'logout-failed'}
  <main class="gate-message"><section><h1>Sign out could not be confirmed</h1><p role="alert">Your private pages are closed, but the server may still recognize this browser session. Retry while connected before another person uses this browser.</p><p>{message}</p><button onclick={signOut}>Retry sign out</button></section></main>
{:else if gate === 'recovery'}
  <RecoveryPage onRecovered={signedIn} />
{:else if gate === 'recovery-unavailable'}
  <main class="gate-message"><section><h1>Recovery is unavailable</h1><p>Configure <code>ADMIN_RECOVERY_TOKEN</code> temporarily, restart NovelReader, and return to <button onclick={() => { gate = 'login'; window.location.hash = '#/login'; }}>sign in</button>.</p></section></main>
{:else if gate === 'authenticated' && account}
  <AuthenticatedApp {account} onLogout={signOut} />
{:else}
  <main class="gate-message"><section><h1>NovelReader could not start</h1><p role="alert">{message}</p><button onclick={bootstrap}>Try again</button></section></main>
{/if}

<style>
  :global(*) { box-sizing: border-box; }
  :global(body) { margin: 0; }
  .gate-message { min-height:100dvh; display:grid; place-items:center; padding:1.5rem; background:#f5f0eb; color:#3a3a3a; font-family:system-ui,sans-serif; text-align:center; }
  .gate-message section { width:min(100%,32rem); padding:2rem; border:1px solid #e5e0db; border-radius:1rem; background:white; }
  h1 { margin:0 0 .75rem; }
  p { line-height:1.55; }
  button { border:0; background:none; color:#5d35c7; font:inherit; font-weight:700; cursor:pointer; }
  .session-message { position:fixed; z-index:2; inset:1rem 1rem auto; margin:auto; width:min(calc(100% - 2rem),28rem); padding:.7rem; border-radius:.5rem; background:#fff2d8; color:#704b13; text-align:center; font:600 .9rem system-ui,sans-serif; }
</style>
