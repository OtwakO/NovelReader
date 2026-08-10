<script lang="ts">
  import { register, type AuthAccount } from '../api/client';

  let { inviteRequired, onComplete, onCancel }: { inviteRequired: boolean; onComplete?: (account: AuthAccount) => void; onCancel?: () => void } = $props();
  let username = $state('');
  let password = $state('');
  let inviteCode = $state('');
  let error = $state('');
  let submitting = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    submitting = true;
    try {
      const account = await register(username, password, inviteCode);
      password = '';
      inviteCode = '';
      onComplete?.(account);
    } catch (cause) {
      password = '';
      error = cause instanceof Error ? cause.message : 'Registration failed.';
    } finally {
      inviteCode = '';
      submitting = false;
    }
  }
</script>

<main class="registration-shell">
  <section class="registration-card" aria-labelledby="registration-title">
    <p class="eyebrow">NovelReader</p>
    <h1 id="registration-title">Create reader account</h1>
    <p class="intro">Your bookshelf, sources, progress, and files will be kept in a private reader home.</p>
    <form onsubmit={submit}>
      <label for="registration-username">Username</label>
      <input id="registration-username" bind:value={username} autocomplete="username" required disabled={submitting} />
      <label for="registration-password">Password</label>
      <input id="registration-password" bind:value={password} type="password" autocomplete="new-password" minlength="12" required disabled={submitting} />
      {#if inviteRequired}
        <label for="registration-invite">Invite code</label>
        <input id="registration-invite" bind:value={inviteCode} type="password" autocomplete="off" required disabled={submitting} />
      {/if}
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button type="submit" disabled={submitting}>{submitting ? 'Creating account…' : 'Create account'}</button>
    </form>
    <button class="cancel" type="button" onclick={() => onCancel?.()}>Back to sign in</button>
  </section>
</main>

<style>
  .registration-shell { min-height:100dvh; display:grid; place-items:center; padding:1.5rem; background:#f5f0eb; color:#3a3a3a; font-family:system-ui,sans-serif; }
  .registration-card { width:min(100%,28rem); padding:2rem; border:1px solid #e5e0db; border-radius:1rem; background:white; box-shadow:0 1rem 2.5rem rgb(58 58 58 / 9%); }
  .eyebrow { margin:0 0 .4rem; color:#7448df; font-size:.75rem; font-weight:800; letter-spacing:.12em; text-transform:uppercase; }
  h1 { margin:0; font-size:2rem; }
  .intro { margin:.7rem 0 1.5rem; color:#68625d; line-height:1.5; }
  form { display:grid; gap:.65rem; }
  label { font-size:.9rem; font-weight:700; }
  input { min-height:2.75rem; padding:.65rem .75rem; border:1px solid #cfc8c1; border-radius:.55rem; font:inherit; }
  input:focus-visible, button:focus-visible { outline:2px solid #8b5cf6; outline-offset:2px; }
  button { min-height:2.75rem; border:0; border-radius:.55rem; background:#8b5cf6; color:white; font:inherit; font-weight:750; cursor:pointer; }
  button:disabled { opacity:.65; cursor:wait; }
  .error { margin:.3rem 0; padding:.7rem; border-radius:.5rem; background:#fff2f2; color:#852c2c; }
  .cancel { display:block; min-height:auto; margin:1rem auto 0; padding:.25rem; background:transparent; color:#5d35c7; font-weight:650; }
</style>
