<script lang="ts">
  import { recoverAdministrator, type AuthAccount, type RecoveryAction } from '../api/client';

  let { onRecovered }: { onRecovered?: (account: AuthAccount) => void } = $props();
  let action: RecoveryAction = $state('reset_existing');
  let token = $state('');
  let username = $state('');
  let password = $state('');
  let confirmation = $state('');
  let error = $state('');
  let submitting = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    if (!token.trim()) {
      error = 'Enter the recovery token from the server environment.';
      return;
    }
    if (!username.trim()) {
      error = 'Enter the Administrator username.';
      return;
    }
    if (password.length < 12) {
      error = 'Use a password with at least 12 characters.';
      return;
    }
    if (password !== confirmation) {
      error = 'The passwords do not match.';
      return;
    }
    submitting = true;
    try {
      const account = await recoverAdministrator(token, action, username, password);
      token = '';
      password = '';
      confirmation = '';
      onRecovered?.(account);
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Administrator recovery failed.';
    } finally {
      submitting = false;
    }
  }
</script>

<main class="recovery-shell">
  <section class="recovery-card" aria-labelledby="recovery-title">
    <p class="eyebrow">NovelReader recovery</p>
    <h1 id="recovery-title">Restore Administrator access</h1>
    <p class="intro">Use the temporary recovery token configured on the server. Remove it from the environment when recovery is complete.</p>

    <form onsubmit={submit}>
      <fieldset disabled={submitting}>
        <legend>Recovery action</legend>
        <label class:chosen={action === 'reset_existing'}>
          <input type="radio" bind:group={action} value="reset_existing" />
          <span><strong>Reset an Administrator</strong><small>Sets a new password, reactivates a disabled account, and signs out every existing session.</small></span>
        </label>
        <label class:chosen={action === 'create_replacement'}>
          <input type="radio" bind:group={action} value="create_replacement" />
          <span><strong>Create a replacement</strong><small>Creates a separate Administrator with a new empty reader home. Existing Reader Data is never attached or changed.</small></span>
        </label>
      </fieldset>

      <label for="recovery-token">Recovery token</label>
      <input id="recovery-token" type="password" autocomplete="off" bind:value={token} disabled={submitting} required />

      <label for="recovery-username">{action === 'reset_existing' ? 'Administrator username' : 'New Administrator username'}</label>
      <input id="recovery-username" autocomplete="username" bind:value={username} disabled={submitting} required />

      <label for="recovery-password">New password</label>
      <input id="recovery-password" type="password" autocomplete="new-password" minlength="12" bind:value={password} disabled={submitting} required />

      <label for="recovery-confirmation">Confirm new password</label>
      <input id="recovery-confirmation" type="password" autocomplete="new-password" minlength="12" bind:value={confirmation} disabled={submitting} required />

      {#if error}
        <p class="error" role="alert">{error}</p>
      {/if}
      <button type="submit" disabled={submitting}>{submitting ? 'Recovering…' : 'Recover and sign in'}</button>
    </form>
  </section>
</main>

<style>
  .recovery-shell { min-height: 100vh; display: grid; place-items: center; padding: 2rem 1rem; background: #f4f1e9; color: #28261f; }
  .recovery-card { width: min(100%, 35rem); padding: clamp(1.5rem, 4vw, 2.75rem); border: 1px solid #d9d2c2; border-radius: 1.25rem; background: #fffdf8; box-shadow: 0 1.25rem 3rem rgb(47 42 31 / 10%); }
  .eyebrow { margin: 0 0 .5rem; color: #8a5a24; font-size: .75rem; font-weight: 700; letter-spacing: .12em; text-transform: uppercase; }
  h1 { margin: 0; font-family: Georgia, serif; font-size: clamp(2rem, 7vw, 3rem); line-height: 1; }
  .intro { margin: 1rem 0 1.5rem; color: #696354; line-height: 1.6; }
  form { display: grid; gap: .65rem; }
  fieldset { display: grid; gap: .6rem; margin: 0 0 .65rem; padding: 0; border: 0; }
  legend, form > label { font-size: .85rem; font-weight: 700; }
  fieldset label { display: flex; gap: .7rem; padding: .85rem; border: 1px solid #d9d2c2; border-radius: .8rem; cursor: pointer; }
  fieldset label.chosen { border-color: #9a6328; background: #faf2e5; }
  fieldset input { width: auto; margin-top: .2rem; }
  fieldset span { display: grid; gap: .25rem; }
  small { color: #696354; font-weight: 400; line-height: 1.4; }
  form > input { width: 100%; box-sizing: border-box; padding: .8rem .9rem; border: 1px solid #cfc7b6; border-radius: .65rem; background: white; color: inherit; font: inherit; }
  form > input:focus { outline: 3px solid rgb(154 99 40 / 18%); border-color: #9a6328; }
  button { margin-top: .8rem; padding: .9rem 1rem; border: 0; border-radius: .7rem; background: #2e4b45; color: white; font: inherit; font-weight: 700; cursor: pointer; }
  button:disabled { cursor: wait; opacity: .65; }
  .error { margin: .35rem 0 0; padding: .75rem; border-radius: .6rem; background: #f8e4df; color: #8a2f24; font-size: .9rem; }
</style>
