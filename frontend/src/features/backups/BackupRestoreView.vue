<script lang="ts">
import { defineComponent } from 'vue';
import { cancelRestore, commitRestore, createBackupToken, downloadBackup, listBackupTokens, prepareRestore, revokeBackupToken, type BackupToken, type BackupTokenCredential, type PreparedRestore } from '../../api/backups';
import AppButton from '../../ui/components/AppButton.vue';
import FeatureScaffold from '../../ui/components/FeatureScaffold.vue';

type ApiExample = 'curl' | 'python' | 'javascript' | 'rest';

const apiExamples: Record<ApiExample, string> = {
  curl: `curl -f -H "Authorization: Bearer $EXPORT_TOKEN" \\
  -o reader-backup.tar.gz \\
  "$NOVELREADER_URL/api/backups/export"

operation_id=$(curl -fsS -X POST \\
  -H "Authorization: Bearer $RESTORE_TOKEN" \\
  -H "Content-Type: application/gzip" \\
  --data-binary @reader-backup.tar.gz \\
  "$NOVELREADER_URL/api/backups/restores" | jq -r .operationId)

curl -f -X POST \\
  -H "Authorization: Bearer $RESTORE_TOKEN" \\
  "$NOVELREADER_URL/api/backups/restores/$operation_id/commit"`,
  python: `import json, os, urllib.request

base = os.environ["NOVELREADER_URL"]
export_token = os.environ["EXPORT_TOKEN"]
restore_token = os.environ["RESTORE_TOKEN"]

request = urllib.request.Request(f"{base}/api/backups/export",
    headers={"Authorization": f"Bearer {export_token}"})
with urllib.request.urlopen(request) as response:
    open("reader-backup.tar.gz", "wb").write(response.read())

request = urllib.request.Request(f"{base}/api/backups/restores",
    data=open("reader-backup.tar.gz", "rb").read(), method="POST",
    headers={"Authorization": f"Bearer {restore_token}",
             "Content-Type": "application/gzip"})
with urllib.request.urlopen(request) as response:
    operation_id = json.load(response)["operationId"]

request = urllib.request.Request(
    f"{base}/api/backups/restores/{operation_id}/commit", method="POST",
    headers={"Authorization": f"Bearer {restore_token}"})
urllib.request.urlopen(request).close()`,
  javascript: `import { readFile, writeFile } from "node:fs/promises";

const base = process.env.NOVELREADER_URL;
const exportHeaders = { Authorization: \`Bearer \${process.env.EXPORT_TOKEN}\` };
const restoreHeaders = { Authorization: \`Bearer \${process.env.RESTORE_TOKEN}\` };

let response = await fetch(\`\${base}/api/backups/export\`, { headers: exportHeaders });
await writeFile("reader-backup.tar.gz", Buffer.from(await response.arrayBuffer()));

response = await fetch(\`\${base}/api/backups/restores\`, {
  method: "POST",
  headers: { ...restoreHeaders, "Content-Type": "application/gzip" },
  body: await readFile("reader-backup.tar.gz"),
});
const { operationId } = await response.json();

await fetch(\`\${base}/api/backups/restores/\${operationId}/commit\`, {
  method: "POST", headers: restoreHeaders,
});`,
  rest: `GET /api/backups/export HTTP/1.1
Host: novelreader.example
Authorization: Bearer <SOURCE_EXPORT_TOKEN>

HTTP/1.1 200 OK
Content-Type: application/gzip

<save response body as reader-backup.tar.gz>

POST /api/backups/restores HTTP/1.1
Host: novelreader.example
Authorization: Bearer <DESTINATION_RESTORE_TOKEN>
Content-Type: application/gzip

<reader-backup.tar.gz bytes>

HTTP/1.1 201 Created
Content-Type: application/json

{"operationId":"<OPERATION_ID>", ...}

POST /api/backups/restores/<OPERATION_ID>/commit HTTP/1.1
Host: novelreader.example
Authorization: Bearer <DESTINATION_RESTORE_TOKEN>`,
};

export default defineComponent({
  name: 'BackupRestoreView', components: { AppButton, FeatureScaffold },
  data() { return { exporting: false, exportError: '', restoreFile: null as File | null, preparing: false, restoreError: '', prepared: null as PreparedRestore | null, confirmation: '', committing: false, tokenLoading: true, tokenError: '', tokens: [] as BackupToken[], tokenName: '', tokenCanExport: true, tokenCanRestore: false, currentPassword: '', tokenExpiry: '', creatingToken: false, revealedToken: null as BackupTokenCredential | null, copied: false, activeApiExample: 'curl' as ApiExample, apiExampleTabs: ['curl', 'python', 'javascript', 'rest'] as ApiExample[] }; },
  computed: {
    canCommit(): boolean { return this.confirmation === this.$t('backups.restore.confirmWord') && !this.committing; },
    apiExampleCode(): string { return apiExamples[this.activeApiExample]; },
  },
  async mounted() { await this.loadTokens(); },
  methods: {
    formatDate(value?: string | number) { if (!value) return ''; const date = typeof value === 'number' ? new Date(value * 1000) : new Date(value); return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date); },
    async exportBackup() { this.exporting = true; this.exportError = ''; try { const { blob, filename } = await downloadBackup(); const url = URL.createObjectURL(blob); const link = document.createElement('a'); link.href = url; link.download = filename; link.click(); URL.revokeObjectURL(url); } catch (cause) { this.exportError = cause instanceof Error ? cause.message : this.$t('backups.export.failed'); } finally { this.exporting = false; } },
    selectRestore(event: Event) { this.restoreFile = (event.target as HTMLInputElement).files?.[0] ?? null; this.restoreError = ''; },
    async prepare() { if (!this.restoreFile) return; this.preparing = true; this.restoreError = ''; try { this.prepared = await prepareRestore(this.restoreFile); this.confirmation = ''; } catch (cause) { this.restoreError = cause instanceof Error ? cause.message : this.$t('backups.restore.failed'); } finally { this.preparing = false; } },
    async cancelPrepared() { if (!this.prepared) return; const operationId = this.prepared.operationId; this.prepared = null; this.confirmation = ''; try { await cancelRestore(operationId); } catch { /* The prepared restore is already unusable locally. */ } },
    async commit() { if (!this.prepared || !this.canCommit) return; this.committing = true; this.restoreError = ''; try { await commitRestore(this.prepared.operationId); window.location.reload(); } catch (cause) { this.restoreError = cause instanceof Error ? cause.message : this.$t('backups.restore.failed'); this.committing = false; } },
    async loadTokens() { this.tokenLoading = true; this.tokenError = ''; try { this.tokens = await listBackupTokens(); } catch (cause) { this.tokenError = cause instanceof Error ? cause.message : ''; } finally { this.tokenLoading = false; } },
    async createToken() { if (!this.tokenName.trim() || (!this.tokenCanExport && !this.tokenCanRestore)) return; this.creatingToken = true; this.tokenError = ''; try { this.revealedToken = await createBackupToken({ name: this.tokenName.trim(), canExport: this.tokenCanExport, canRestore: this.tokenCanRestore, currentPassword: this.tokenCanRestore ? this.currentPassword : undefined, expiresAt: this.tokenExpiry ? Math.floor(new Date(this.tokenExpiry).getTime() / 1000) : undefined }); this.tokens = [this.revealedToken, ...this.tokens]; this.tokenName = ''; this.currentPassword = ''; this.tokenExpiry = ''; this.copied = false; } catch (cause) { this.tokenError = cause instanceof Error ? cause.message : ''; } finally { this.creatingToken = false; } },
    async copyToken() { if (!this.revealedToken) return; await navigator.clipboard.writeText(this.revealedToken.token); this.copied = true; },
    async revoke(tokenId: string) { try { await revokeBackupToken(tokenId); this.tokens = this.tokens.filter((token) => token.id !== tokenId); if (this.revealedToken?.id === tokenId) this.revealedToken = null; } catch (cause) { this.tokenError = cause instanceof Error ? cause.message : ''; } },
  },
});
</script>

<template>
  <FeatureScaffold :title="$t('backups.title')" :description="$t('backups.description')">
    <div class="backup-page">
      <section class="panel export-panel">
        <header>
          <div><h2>{{ $t('backups.export.title') }}</h2><p>{{ $t('backups.export.description') }}</p></div>
          <AppButton :busy="exporting" @click="exportBackup">{{ exporting ? $t('backups.export.busy') : $t('backups.export.action') }}</AppButton>
        </header>
        <div class="included-data"><strong>{{ $t('backups.export.included') }}</strong><div class="data-tags"><span v-for="item in $tm('backups.export.items')" :key="String(item)">{{ item }}</span></div></div>
        <p v-if="exportError" class="error" role="alert">{{ exportError }}</p>
      </section>

      <section class="panel restore-panel">
        <header><div><h2>{{ $t('backups.restore.title') }}</h2><p>{{ $t('backups.restore.description') }}</p></div></header>
        <div v-if="!prepared" class="restore-upload">
          <label class="file-control"><strong>{{ $t('backups.restore.choose') }}</strong><span v-if="restoreFile" class="file-name">{{ restoreFile.name }}</span><input class="file-input" type="file" accept=".tar.gz,.tgz,application/gzip,application/x-gzip,application/octet-stream" :disabled="preparing" @change="selectRestore"></label>
          <AppButton :disabled="!restoreFile" :busy="preparing" @click="prepare">{{ preparing ? $t('backups.restore.preparing') : $t('backups.restore.prepare') }}</AppButton>
        </div>
        <div v-else class="restore-ready">
          <div class="ready-heading"><div><h3>{{ $t('backups.restore.ready') }}</h3><p>{{ $t('backups.restore.source', { username: prepared.exportedFromUsername }) }}</p><p>{{ $t('backups.restore.created', { date: formatDate(prepared.createdAt) }) }}</p><p>{{ $t('backups.restore.schema', { version: prepared.readerSchemaVersion }) }} · {{ $t('backups.restore.expires', { date: formatDate(prepared.expiresAt) }) }}</p></div></div>
          <p class="warning">{{ $t('backups.restore.warning') }}</p>
          <label class="confirmation">{{ $t('backups.restore.confirmLabel') }}<input v-model="confirmation" placeholder="RESTORE" autocomplete="off" :disabled="committing"></label>
          <div class="actions"><AppButton variant="danger" :disabled="!canCommit" :busy="committing" @click="commit">{{ committing ? $t('backups.restore.committing') : $t('backups.restore.commit') }}</AppButton><AppButton variant="quiet" :disabled="committing" @click="cancelPrepared">{{ $t('backups.restore.cancel') }}</AppButton></div>
        </div>
        <p v-if="restoreError" class="error" role="alert">{{ restoreError }}</p>
      </section>

      <section class="panel tokens-panel">
        <header><div><h2>{{ $t('backups.tokens.title') }}</h2><p>{{ $t('backups.tokens.description') }}</p></div></header>
        <form class="token-form" @submit.prevent="createToken">
          <label>{{ $t('backups.tokens.name') }}<input v-model="tokenName" required maxlength="80" :disabled="creatingToken"></label>
          <fieldset><legend class="sr-only">Scopes</legend><label class="scope"><input v-model="tokenCanExport" type="checkbox">{{ $t('backups.tokens.exportScope') }}</label><label class="scope"><input v-model="tokenCanRestore" type="checkbox">{{ $t('backups.tokens.restoreScope') }}</label></fieldset>
          <label>{{ $t('backups.tokens.expiry') }}<input v-model="tokenExpiry" type="datetime-local"><small>{{ $t('backups.tokens.expiryHint') }}</small></label>
          <label v-if="tokenCanRestore">{{ $t('backups.tokens.password') }}<input v-model="currentPassword" type="password" autocomplete="current-password" required maxlength="128"><small>{{ $t('backups.tokens.passwordHint') }}</small></label>
          <AppButton type="submit" :disabled="!tokenName.trim() || (!tokenCanExport && !tokenCanRestore)" :busy="creatingToken">{{ creatingToken ? $t('backups.tokens.creating') : $t('backups.tokens.create') }}</AppButton>
        </form>
        <div v-if="revealedToken" class="token-secret"><div><h3>{{ $t('backups.tokens.secretTitle') }}</h3><p>{{ $t('backups.tokens.secretDescription') }}</p></div><div class="secret-value"><code>{{ revealedToken.token }}</code><AppButton variant="quiet" @click="copyToken">{{ copied ? $t('backups.tokens.copied') : $t('backups.tokens.copy') }}</AppButton></div></div>
        <p v-if="tokenError" class="error" role="alert">{{ tokenError }}</p>
        <p v-if="tokenLoading" class="muted">{{ $t('backups.tokens.loading') }}</p>
        <p v-else-if="tokens.length === 0" class="muted empty-state">{{ $t('backups.tokens.empty') }}</p>
        <ul v-else class="token-list"><li v-for="token in tokens" :key="token.id"><div><div class="token-title"><strong>{{ token.name }}</strong><span class="scopes"><em v-if="token.canExport">{{ $t('backups.tokens.export') }}</em><em v-if="token.canRestore">{{ $t('backups.tokens.restore') }}</em></span></div><small>{{ $t('backups.tokens.created', { date: formatDate(token.createdAt) }) }} · {{ token.lastUsedAt ? $t('backups.tokens.lastUsed', { date: formatDate(token.lastUsedAt) }) : $t('backups.tokens.neverUsed') }} · {{ token.expiresAt ? $t('backups.tokens.expires', { date: formatDate(token.expiresAt) }) : $t('backups.tokens.noExpiry') }}</small></div><AppButton variant="quiet" @click="revoke(token.id)">{{ $t('backups.tokens.revoke') }}</AppButton></li></ul>
      </section>

      <section class="panel api-panel">
        <header><div><h2>{{ $t('backups.api.title') }}</h2><p>{{ $t('backups.api.description') }}</p></div></header>
        <details>
          <summary>{{ $t('backups.api.show') }}</summary>
          <div class="api-docs">
            <p class="api-note">{{ $t('backups.api.auth') }}</p>
            <div class="endpoint-list" role="list">
              <div role="listitem"><code>GET /api/backups/export</code><span><strong>backup:export</strong> · {{ $t('backups.api.export') }}</span></div>
              <div role="listitem"><code>POST /api/backups/restores</code><span><strong>backup:restore</strong> · {{ $t('backups.api.prepare') }}</span></div>
              <div role="listitem"><code>GET /api/backups/restores/{operationId}</code><span><strong>backup:restore</strong> · {{ $t('backups.api.status') }}</span></div>
              <div role="listitem"><code>POST /api/backups/restores/{operationId}/commit</code><span><strong>backup:restore</strong> · {{ $t('backups.api.commit') }}</span></div>
              <div role="listitem"><code>DELETE /api/backups/restores/{operationId}</code><span><strong>backup:restore</strong> · {{ $t('backups.api.cancel') }}</span></div>
            </div>
            <h3>{{ $t('backups.api.exampleTitle') }}</h3>
            <p>{{ $t('backups.api.exampleDescription') }}</p>
            <div class="example-tabs" role="tablist" :aria-label="$t('backups.api.exampleTabs')">
              <button v-for="tab in apiExampleTabs" :id="`api-example-${tab}`" :key="tab" type="button" role="tab" :aria-selected="activeApiExample === tab" :aria-controls="'api-example-panel'" @click="activeApiExample = tab">{{ tab === 'javascript' ? 'JavaScript' : tab === 'python' ? 'Python' : tab === 'rest' ? 'REST' : 'cURL' }}</button>
            </div>
            <pre id="api-example-panel" role="tabpanel" :aria-labelledby="`api-example-${activeApiExample}`"><code>{{ apiExampleCode }}</code></pre>
            <p class="api-note">{{ $t('backups.api.note') }}</p>
          </div>
        </details>
      </section>
    </div>
  </FeatureScaffold>
</template>

<style scoped>
.backup-page { display: grid; gap: 1rem; }
.panel { padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.panel > header { display: flex; align-items: center; justify-content: space-between; gap: 1.5rem; }
.panel h2 { margin: .15rem 0; font: 700 1.25rem var(--font-literary); }
.panel header p, .muted { margin: .25rem 0 0; color: var(--color-ink-muted); line-height: 1.55; }
.included-data { display: flex; align-items: center; gap: .75rem; margin-top: .9rem; color: var(--color-ink-muted); font-size: .82rem; }
.data-tags { display: flex; flex-wrap: wrap; gap: .4rem; }
.data-tags span, .scopes em { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: .15rem .45rem; background: var(--color-paper-muted); color: var(--color-accent); font-size: .72rem; font-style: normal; font-weight: 750; }
.restore-upload { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: .75rem; align-items: end; margin-top: 1rem; }
.file-control { position: relative; min-height: 7.5rem; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: .4rem; border: 1px dashed var(--color-border); border-radius: var(--radius-md); padding: 1rem; background: var(--color-paper-muted); text-align: center; cursor: pointer; }
.file-control:hover { border-color: var(--color-accent); background: var(--color-accent-soft); }
.file-control:focus-within { outline: 3px solid color-mix(in srgb, var(--color-accent) 30%, transparent); outline-offset: 2px; }
.file-input { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); clip-path: inset(50%); white-space: nowrap; }
.file-name { max-width: 100%; color: var(--color-ink-muted); font-size: .82rem; overflow-wrap: anywhere; }
.restore-ready { display: grid; gap: .85rem; max-width: 44rem; margin-top: 1rem; padding: 1rem; border: 1px solid color-mix(in srgb, var(--color-danger) 25%, var(--color-border)); border-radius: var(--radius-md); background: var(--color-paper-muted); }
.ready-heading h3, .token-secret h3 { margin: 0; font: 700 1.1rem var(--font-literary); }
.ready-heading p { margin: .25rem 0 0; color: var(--color-ink-muted); font-size: .85rem; }
.warning, .error { margin: 0; padding: .7rem; border-radius: var(--radius-md); }
.warning { background: #fae9e6; color: var(--color-danger); font-weight: 700; line-height: 1.5; }
.error { margin-top: .75rem; background: #f8e4df; color: var(--color-danger); }
.confirmation, .token-form > label { display: grid; gap: .35rem; font-size: .8rem; font-weight: 700; }
.confirmation input, .token-form > label input { min-height: 2.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .6rem .7rem; background: white; color: var(--color-ink); }
.confirmation input { max-width: 18rem; }
.actions { display: flex; flex-wrap: wrap; gap: .5rem; }
.token-form { max-width: 40rem; display: grid; gap: .85rem; margin-top: 1rem; }
.token-form fieldset { display: flex; flex-wrap: wrap; gap: .55rem; margin: 0; padding: 0; border: 0; }
.scope { min-height: 2.75rem; display: flex; align-items: center; gap: .5rem; padding: .5rem .7rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-paper-muted); font-weight: 650; }
.scope input { width: 1.1rem; height: 1.1rem; }
.token-form small { color: var(--color-ink-muted); font-weight: 400; }
.token-form :deep(.app-button) { justify-self: start; }
.token-secret { display: grid; gap: .7rem; max-width: 40rem; margin-top: 1rem; padding: .85rem; border: 1px solid color-mix(in srgb, var(--color-accent) 45%, var(--color-border)); border-radius: var(--radius-md); background: var(--color-accent-soft); }
.token-secret p { margin: .25rem 0 0; color: var(--color-ink-muted); }
.secret-value { display: flex; align-items: center; gap: .5rem; padding: .35rem .4rem .35rem .7rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: white; }
.secret-value code { min-width: 0; flex: 1; overflow-wrap: anywhere; user-select: all; }
.token-list { display: grid; gap: .55rem; margin: 1rem 0 0; padding: 0; list-style: none; }
.token-list li { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: .75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); }
.token-title { display: flex; align-items: center; flex-wrap: wrap; gap: .5rem; }
.scopes { display: inline-flex; gap: .35rem; }
.token-list small { display: block; margin-top: .3rem; color: var(--color-ink-muted); }
.empty-state { margin-top: 1rem; padding: 1rem; text-align: center; }
.api-panel details { margin-top: .9rem; }
.api-panel summary { width: fit-content; color: var(--color-accent); font-weight: 750; cursor: pointer; }
.api-panel summary:focus-visible { outline: 3px solid color-mix(in srgb, var(--color-accent) 30%, transparent); outline-offset: 3px; border-radius: var(--radius-sm); }
.api-docs { display: grid; gap: .85rem; margin-top: 1rem; }
.api-docs h3 { margin: .35rem 0 -.45rem; font: 700 1rem var(--font-literary); }
.api-docs p { margin: 0; color: var(--color-ink-muted); line-height: 1.55; }
.api-note { max-width: 72ch; }
.endpoint-list { display: grid; border: 1px solid var(--color-border); border-radius: var(--radius-md); overflow: hidden; }
.endpoint-list > div { display: grid; grid-template-columns: minmax(18rem, .9fr) minmax(0, 1fr); gap: 1rem; padding: .7rem; background: var(--color-paper-muted); }
.endpoint-list > div + div { border-top: 1px solid var(--color-border); }
.endpoint-list code { color: var(--color-ink); font-size: .8rem; overflow-wrap: anywhere; }
.endpoint-list span { color: var(--color-ink-muted); font-size: .82rem; }
.endpoint-list strong { color: var(--color-accent); }
.example-tabs { display: flex; gap: .25rem; margin-bottom: -.85rem; padding: .25rem .25rem 0; border: 1px solid var(--color-border); border-bottom: 0; border-radius: var(--radius-md) var(--radius-md) 0 0; background: var(--color-paper-muted); overflow-x: auto; }
.example-tabs button { min-height: 2.4rem; border: 0; border-radius: var(--radius-sm) var(--radius-sm) 0 0; padding: .45rem .75rem; background: transparent; color: var(--color-ink-muted); font: inherit; font-size: .8rem; font-weight: 700; white-space: nowrap; cursor: pointer; }
.example-tabs button:hover { color: var(--color-ink); background: color-mix(in srgb, var(--color-accent) 8%, transparent); }
.example-tabs button[aria-selected="true"] { background: #26343a; color: #f8f3e7; }
.example-tabs button:focus-visible { outline: 3px solid color-mix(in srgb, var(--color-accent) 35%, transparent); outline-offset: -1px; }
.api-docs pre { max-width: 100%; margin: 0; padding: .85rem; border: 1px solid var(--color-border); border-radius: 0 0 var(--radius-md) var(--radius-md); background: #26343a; color: #f8f3e7; overflow-x: auto; font-size: .78rem; line-height: 1.55; tab-size: 2; }
.api-docs pre code { user-select: all; white-space: pre; }
@media (max-width: 42rem) {
  .panel > header { align-items: stretch; flex-direction: column; gap: .75rem; }
  .panel > header :deep(.app-button) { width: 100%; }
  .included-data { align-items: flex-start; flex-direction: column; }
  .restore-upload { grid-template-columns: 1fr; }
  .restore-upload :deep(.app-button), .token-form :deep(.app-button) { width: 100%; }
  .confirmation input { max-width: none; }
  .actions { display: grid; grid-template-columns: 1fr 1fr; }
  .secret-value { align-items: stretch; flex-direction: column; }
  .token-list li { align-items: flex-start; }
  .token-list :deep(.app-button) { flex: none; }
  .endpoint-list > div { grid-template-columns: 1fr; gap: .35rem; }
  .api-docs pre { font-size: .72rem; }
}
</style>
