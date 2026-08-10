<script lang="ts">
  import { createInitialAdministrator } from '../api/client';

  let { onComplete = () => {} }: { onComplete?: (username: string) => void } = $props();

  let token = $state('');
  let username = $state('');
  let password = $state('');
  let confirmPassword = $state('');
  let submitting = $state(false);
  let error = $state('');

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    if (password !== confirmPassword) {
      error = 'Passwords do not match.';
      return;
    }
    submitting = true;
    try {
      const account = await createInitialAdministrator(token, username, password);
      token = '';
      password = '';
      confirmPassword = '';
      onComplete(account.username);
    } catch (requestError) {
      error = requestError instanceof Error ? requestError.message : 'Setup could not be completed.';
    } finally {
      submitting = false;
    }
  }
</script>

<section class="setup-shell" aria-labelledby="setup-title">
  <div class="setup-panel">
    <h1 id="setup-title">Set up NovelReader</h1>
    <p class="intro">Create the first Administrator account for this installation.</p>

    <form onsubmit={submit}>
      <label>
        Bootstrap token
        <input bind:value={token} type="password" name="bootstrap-token" autocomplete="off" required disabled={submitting} />
      </label>
      <p class="hint">Paste the temporary value configured as <code>ADMIN_BOOTSTRAP_TOKEN</code>.</p>

      <label>
        Username
        <input bind:value={username} name="username" autocomplete="username" minlength="3" maxlength="32" required disabled={submitting} />
      </label>

      <label>
        Password
        <input bind:value={password} type="password" name="password" autocomplete="new-password" minlength="12" maxlength="128" required disabled={submitting} />
      </label>

      <label>
        Confirm password
        <input bind:value={confirmPassword} type="password" name="confirm-password" autocomplete="new-password" minlength="12" maxlength="128" required disabled={submitting} />
      </label>

      {#if error}
        <p class="error" role="alert">{error}</p>
      {/if}

      <button type="submit" disabled={submitting}>
        {submitting ? 'Creating account…' : 'Create Administrator'}
      </button>
    </form>

    <p class="aftercare">After setup succeeds, remove <code>ADMIN_BOOTSTRAP_TOKEN</code> from the server environment.</p>
  </div>
</section>

<style>
  .setup-shell {
    min-height: 100dvh;
    display: grid;
    place-items: center;
    padding: 1.5rem;
    background: #f5f0eb;
    color: #3a3a3a;
    font-family: system-ui, -apple-system, sans-serif;
  }

  .setup-panel {
    width: min(100%, 28rem);
    padding: 2rem;
    border: 1px solid #e5e0db;
    border-radius: 12px;
    background: #fff;
    box-shadow: 0 12px 32px rgb(58 58 58 / 0.08);
  }

  h1 { font-size: 1.5rem; line-height: 1.2; margin: 0; }
  .intro { margin: 0.6rem 0 1.75rem; color: #5f5a56; line-height: 1.5; }
  form { display: grid; gap: 1rem; }
  label { display: grid; gap: 0.4rem; font-weight: 600; font-size: 0.9rem; }
  input {
    min-height: 2.75rem;
    width: 100%;
    padding: 0.65rem 0.75rem;
    border: 1px solid #cfc8c1;
    border-radius: 8px;
    background: #fff;
    color: inherit;
    font: inherit;
  }
  input:focus-visible { outline: 2px solid #8b5cf6; outline-offset: 2px; border-color: #8b5cf6; }
  input:disabled { background: #f1eeeb; color: #77716c; }
  .hint { margin: -0.55rem 0 0; color: #68625d; font-size: 0.82rem; line-height: 1.45; }
  code { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 0.9em; }
  .error {
    margin: 0;
    padding: 0.75rem;
    border: 1px solid #d9a2a2;
    border-radius: 8px;
    background: #fff2f2;
    color: #852c2c;
    line-height: 1.4;
  }
  button {
    min-height: 2.75rem;
    border: 0;
    border-radius: 8px;
    padding: 0.7rem 1rem;
    background: #8b5cf6;
    color: #fff;
    font: inherit;
    font-weight: 700;
    cursor: pointer;
  }
  button:hover:not(:disabled) { background: #7448df; }
  button:focus-visible { outline: 2px solid #5d35c7; outline-offset: 3px; }
  button:disabled { background: #aaa2b8; cursor: wait; }
  .aftercare { margin: 1.5rem 0 0; color: #68625d; font-size: 0.85rem; line-height: 1.5; }

  @media (max-width: 32rem) {
    .setup-shell { align-items: start; padding: 0; }
    .setup-panel { min-height: 100dvh; border: 0; border-radius: 0; padding: 1.5rem; box-shadow: none; }
  }
</style>
