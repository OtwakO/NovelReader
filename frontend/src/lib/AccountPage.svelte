<script lang="ts">
  import { onMount } from 'svelte';
  import { changePassword, listReaderAccounts, setReaderEnabled, type AdminReaderAccount, type AuthAccount } from '../api/client';
  import { clearedPasswordFields } from './accountGate.mjs';
  import { mayManageReaders, readerStatusControl } from './readerAdministration.mjs';

  let { account, onPasswordChanged, onLogout }: { account: AuthAccount; onPasswordChanged?: () => void; onLogout?: () => void } = $props();
  let currentPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let error = $state('');
  let submitting = $state(false);
  let readers: AdminReaderAccount[] = $state([]);
  let readersLoading = $state(false);
  let readersError = $state('');
  let changingReaderID = $state('');

  onMount(() => {
    if (mayManageReaders(account.role)) loadReaders();
  });

  async function loadReaders() {
    readersLoading = true;
    readersError = '';
    try {
      readers = await listReaderAccounts();
    } catch (cause) {
      readersError = cause instanceof Error ? cause.message : 'Reader accounts could not be loaded.';
    } finally {
      readersLoading = false;
    }
  }

  async function changeReaderStatus(reader: AdminReaderAccount) {
    const control = readerStatusControl(reader.status);
    if (!control.available || changingReaderID !== '') return;
    if (control.confirmDisable && !window.confirm(`Disable ${reader.username}? Every browser session for this reader will be signed out.`)) return;
    const enabled = control.enabled;
    changingReaderID = reader.id;
    readersError = '';
    try {
      const updated = await setReaderEnabled(reader.id, enabled);
      readers = readers.map((candidate) => candidate.id === updated.id ? updated : candidate);
    } catch (cause) {
      readersError = cause instanceof Error ? cause.message : 'Reader account could not be updated.';
    } finally {
      changingReaderID = '';
    }
  }

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

  {#if mayManageReaders(account.role)}
    <section class="reader-section" aria-labelledby="reader-title">
      <div class="section-heading">
        <div><h3 id="reader-title">Reader accounts</h3><p>Disabling a reader signs out every browser without changing their password or data.</p></div>
        <button class="secondary compact" type="button" onclick={loadReaders} disabled={readersLoading}>Refresh</button>
      </div>
      {#if readersError}<p class="error" role="alert">{readersError}</p>{/if}
      {#if readersLoading}
        <p class="muted">Loading reader accounts…</p>
      {:else if readers.length === 0}
        <p class="muted">No ordinary reader accounts yet.</p>
      {:else}
        <ul class="reader-list">
          {#each readers as reader (reader.id)}
            {@const control = readerStatusControl(reader.status)}
            <li>
              <div><strong>{reader.username}</strong><span class:disabled-status={reader.status === 'disabled'}>{reader.status}</span></div>
              <button class:danger={control.confirmDisable} class="compact" type="button" disabled={changingReaderID !== '' || !control.available} onclick={() => changeReaderStatus(reader)}>
                {changingReaderID === reader.id ? 'Updating…' : control.label}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {/if}

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
  .session-section, .reader-section { margin-top:1rem; }
  .secondary { padding-inline:1rem; border:1px solid var(--border); background:white; color:inherit; }
  .section-heading { display:flex; align-items:flex-start; justify-content:space-between; gap:1rem; }
  .section-heading h3 { margin-bottom:.25rem; }
  .section-heading p, .muted { color:#746e69; line-height:1.45; }
  .reader-list { display:grid; gap:.65rem; margin:1rem 0 0; padding:0; list-style:none; }
  .reader-list li { display:flex; align-items:center; justify-content:space-between; gap:1rem; padding:.8rem; border:1px solid var(--border); border-radius:.6rem; }
  .reader-list li div { display:grid; gap:.2rem; min-width:0; }
  .reader-list span { color:#39704a; font-size:.78rem; font-weight:800; letter-spacing:.05em; text-transform:uppercase; }
  .reader-list .disabled-status { color:#8b5a24; }
  .compact { min-height:2.25rem; padding:.4rem .75rem; }
  .danger { background:#8a2f2f; }
  @media (max-width: 30rem) { .section-heading, .reader-list li { align-items:stretch; flex-direction:column; } .compact { width:100%; } }
</style>
