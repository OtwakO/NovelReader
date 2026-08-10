<script lang="ts">
  import { completePasswordReset } from '../api/client';
  import { clearedResetFields, passwordsMatch } from './passwordReset.mjs';

  let { onComplete, onCancel }: { onComplete?: () => void; onCancel?: () => void } = $props();
  let token = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let error = $state('');
  let submitting = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    if (!passwordsMatch(newPassword, confirmPassword)) {
      error = 'New passwords do not match.';
      return;
    }
    submitting = true;
    try {
      await completePasswordReset(token, newPassword);
      onComplete?.();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Password reset failed.';
    } finally {
      const cleared = clearedResetFields();
      token = cleared.token;
      newPassword = cleared.newPassword;
      confirmPassword = cleared.confirmPassword;
      submitting = false;
    }
  }
</script>

<main class="reset-shell">
  <section class="reset-card" aria-labelledby="reset-title">
    <p class="eyebrow">NovelReader</p>
    <h1 id="reset-title">Reset reader password</h1>
    <p class="intro">Enter the one-time token from your Administrator and choose a new password. The token expires after 30 minutes.</p>
    <form onsubmit={submit}>
      <label for="reset-token">Reset token</label>
      <input id="reset-token" bind:value={token} autocomplete="off" required disabled={submitting} />
      <label for="reset-new-password">New password</label>
      <input id="reset-new-password" bind:value={newPassword} type="password" autocomplete="new-password" minlength="12" required disabled={submitting} />
      <label for="reset-confirm-password">Confirm new password</label>
      <input id="reset-confirm-password" bind:value={confirmPassword} type="password" autocomplete="new-password" minlength="12" required disabled={submitting} />
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button type="submit" disabled={submitting}>{submitting ? 'Resetting password…' : 'Reset password'}</button>
    </form>
    <button class="secondary" type="button" onclick={() => onCancel?.()} disabled={submitting}>Return to sign in</button>
  </section>
</main>

<style>
  .reset-shell { min-height:100dvh; display:grid; place-items:center; padding:1.5rem; background:#f5f0eb; color:#3a3a3a; font-family:system-ui,sans-serif; }
  .reset-card { width:min(100%,28rem); padding:2rem; border:1px solid #e5e0db; border-radius:1rem; background:white; box-shadow:0 1rem 2.5rem rgb(58 58 58 / 9%); }
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
  .secondary { width:100%; margin-top:.75rem; background:transparent; color:#5d35c7; }
</style>
