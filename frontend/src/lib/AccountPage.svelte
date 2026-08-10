<script lang="ts">
  import { changePassword, type AuthAccount } from '../api/client';
  import { clearedPasswordFields } from './accountGate.mjs';

  let { account, onPasswordChanged, onLogout }: { account: AuthAccount; onPasswordChanged?: () => void; onLogout?: () => void } = $props();
  let currentPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let error = $state('');
  let submitting = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    if (newPassword !== confirmPassword) {
      error = 'New passwords do not match.';
      return;
    }
    submitting = true;
    try {
      await changePassword(currentPassword, newPassword);
      onPasswordChanged?.();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Password change failed.';
    } finally {
      const cleared = clearedPasswordFields();
      currentPassword = cleared.currentPassword;
      newPassword = cleared.newPassword;
      confirmPassword = cleared.confirmPassword;
      submitting = false;
    }
  }
</script>

<div class="page">
  <header>
    <p class="eyebrow">Account</p>
    <h2>{account.username}</h2>
    <p>Your password is shared by no reader data. Changing it signs out every browser session.</p>
  </header>

  <section aria-labelledby="password-title">
    <h3 id="password-title">Change password</h3>
    <form onsubmit={submit}>
      <label for="current-password">Current password</label>
      <input id="current-password" bind:value={currentPassword} type="password" autocomplete="current-password" required disabled={submitting} />
      <label for="new-password">New password</label>
      <input id="new-password" bind:value={newPassword} type="password" autocomplete="new-password" minlength="12" required disabled={submitting} />
      <label for="confirm-password">Confirm new password</label>
      <input id="confirm-password" bind:value={confirmPassword} type="password" autocomplete="new-password" minlength="12" required disabled={submitting} />
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button type="submit" disabled={submitting}>{submitting ? 'Changing password…' : 'Change password'}</button>
    </form>
  </section>

  <section class="session-section">
    <h3>Current session</h3>
    <button class="secondary" type="button" onclick={() => onLogout?.()}>Sign out</button>
  </section>
</div>

<style>
  .page { width:min(100%,34rem); margin:0 auto; padding:1.5rem 1rem 3rem; }
  header { margin-bottom:1.5rem; }
  .eyebrow { margin:0 0 .25rem; color:var(--accent); font-size:.75rem; font-weight:800; letter-spacing:.12em; text-transform:uppercase; }
  h2 { font-size:1.6rem; }
  header p:last-child { margin-top:.5rem; color:#746e69; line-height:1.5; }
  section { padding:1.25rem; border:1px solid var(--border); border-radius:.8rem; background:var(--card-bg); }
  h3 { margin-bottom:1rem; font-size:1rem; }
  form { display:grid; gap:.65rem; }
  label { font-size:.88rem; font-weight:700; }
  input { min-height:2.75rem; padding:.65rem .75rem; border:1px solid #cfc8c1; border-radius:.55rem; font:inherit; }
  input:focus-visible, button:focus-visible { outline:2px solid var(--accent); outline-offset:2px; }
  button { min-height:2.75rem; border:0; border-radius:.55rem; background:var(--accent); color:white; font:inherit; font-weight:750; cursor:pointer; }
  button:disabled { opacity:.65; cursor:wait; }
  .error { margin:.3rem 0; padding:.7rem; border-radius:.5rem; background:#fff2f2; color:#852c2c; }
  .session-section { margin-top:1rem; }
  .secondary { padding-inline:1rem; border:1px solid var(--border); background:white; color:inherit; }
</style>
