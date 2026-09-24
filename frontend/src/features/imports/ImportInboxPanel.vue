<script lang="ts">
import { defineComponent } from 'vue';
import AppButton from '../../ui/components/AppButton.vue';
import { cancelInboxReview, listInboxClaims, resolveInbox, reviewInbox, scanInbox, type ImportFormat, type InboxClaim, type InboxEntry, type InboxReview } from '../../api/file-imports';
import { ApiError } from '../../api/transport';
import { useImportQueue } from './import-queue';
import { createImportTask } from './import-task';
import { importErrorKey } from './import-feedback';
import { useImportPreferences } from './import-preferences';
export default defineComponent({
  components: { AppButton },
  data: () => ({
    queue: useImportQueue(), task: createImportTask(), entries: [] as InboxEntry[], claims: [] as InboxClaim[],
    format: 'txt' as ImportFormat, preferences: useImportPreferences(), directory: '', after: '', next: '', claimAfter: '', claimNext: '', selected: [] as string[], proof: undefined as InboxReview | undefined,
  }),
  watch: { 'queue.revision'() { this.scan(); } },
  mounted() { this.scan(); },
  beforeUnmount() { this.task.cancel(); },
  methods: {
    importErrorKey,
    async load(signal: AbortSignal) {
      const [scan, pending] = await Promise.all([scanInbox(this.format, this.after, signal), listInboxClaims(this.format, this.claimAfter, signal)]);
      signal.throwIfAborted();
      this.entries = scan.items; this.directory = scan.directory; this.next = scan.nextCursor || '';
      this.claims = pending.items; this.claimNext = pending.nextCursor || '';
      this.selected = this.selected.filter(name => scan.items.some(item => item.name === name && !item.problem && !item.receiptId));
    },
    changeFormat(format: ImportFormat) {
      void this.task.run(async signal => {
        await this.abandon(signal);
        this.format = format; this.entries = []; this.claims = []; this.selected = [];
        this.after = ''; this.next = ''; this.claimAfter = ''; this.claimNext = '';
        await this.load(signal);
      });
    },
    scan() { void this.task.run(this.load); },
    async abandon(signal: AbortSignal) {
      const token = this.proof?.token; this.proof = undefined;
      if (token) {
        try { await cancelInboxReview(this.format, token, signal); }
        catch (cause) { if (!(cause instanceof ApiError && cause.code === `${this.format}_inbox_review_expired`)) throw cause; }
      }
    },
    review(id: string) { void this.task.run(async signal => { await this.abandon(signal); this.proof = await reviewInbox(this.format, id, signal); }); },
    resolve(action: 'confirm' | 'release') {
      const token = this.proof?.token;
      if (!token) return;
      void this.task.run(async signal => {
        this.proof = undefined; // One attempt consumes approval, even when its response is lost.
        await resolveInbox(this.format, token, action, signal); await this.load(signal);
      });
    },
  },
});
</script>

<template>
  <section class="import-section" aria-labelledby="inbox-title">
    <header class="import-panel-heading">
      <h2 id="inbox-title">{{ $t('imports.inbox') }}</h2>
      <AppButton variant="secondary" :busy="task.busy" @click="scan">{{ $t('imports.scan') }}</AppButton>
    </header>
    <div class="import-filters">
      <label>{{ $t('imports.format') }}<select :value="format" :disabled="task.busy" @change="changeFormat(($event.target as HTMLSelectElement).value as ImportFormat)"><option value="txt">TXT</option><option value="epub">EPUB</option></select></label>
    </div>
    <div v-if="format === 'epub'">
      <label class="import-selection"><input v-model="preferences.optimizeEPUBImages" type="checkbox" aria-describedby="inbox-image-hint">{{ $t('imports.epub.optimize') }}</label>
      <p id="inbox-image-hint" class="import-note">{{ $t('imports.epub.optimizeHint') }}</p>
    </div>
    <p class="import-note">{{ $t('imports.inboxHint') }}</p>
    <p v-if="directory" class="import-directory"><span>{{ $t('imports.inboxDirectory') }}</span><code>DATA_DIR/{{ directory }}</code></p>
    <p v-if="task.error" role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
    <div class="import-inbox-layout">
      <section class="import-inbox-files" aria-labelledby="inbox-files-title" :aria-busy="task.busy">
        <h3 id="inbox-files-title">{{ $t('imports.inboxFiles') }}</h3>
        <p v-if="!entries.length && !task.busy" class="import-empty">{{ $t('imports.inboxEmpty') }}</p>
        <ul class="import-list import-inbox-list">
          <li v-for="item in entries" :key="item.name">
            <label class="import-inbox-file">
              <input v-model="selected" type="checkbox" :value="item.name" :disabled="!!item.problem || !!item.receiptId || task.busy">
              <span class="import-copy"><strong>{{ item.name }}</strong>
                <small v-if="item.problem">{{ $t('imports.unavailableFile') }} ({{ item.problem }})</small>
                <small v-else-if="item.receiptId">{{ $t('imports.claimRequired') }}</small>
              </span>
            </label>
          </li>
        </ul>
        <div v-if="entries.length" class="app-actions app-actions--end import-actions">
          <AppButton :disabled="!selected.length || task.busy" @click="queue.enqueue(selected, preferences.optimizeEPUBImages ? 'optimized' : 'original'); selected = []">{{ $t('imports.acquireSelected', { count: selected.length }) }}</AppButton>
        </div>
        <div v-if="after || next" class="app-actions import-actions">
          <AppButton variant="quiet" :disabled="!after || task.busy" @click="after = ''; selected = []; scan()">{{ $t('imports.first') }}</AppButton>
          <AppButton variant="quiet" :disabled="!next || task.busy" @click="after = next; selected = []; scan()">{{ $t('imports.next') }}</AppButton>
        </div>
      </section>
      <section class="import-inbox-recovery" aria-labelledby="inbox-recovery-title">
        <h3 id="inbox-recovery-title">{{ $t('imports.leftovers') }}</h3>
        <p class="import-note">{{ $t('imports.leftoversHint') }}</p>
        <p v-if="!claims.length && !task.busy" class="import-empty">{{ $t('imports.noClaims') }}</p>
        <ul class="import-list">
          <li v-for="claim in claims" :key="claim.receiptId"><strong class="import-copy">{{ claim.name }}</strong><AppButton variant="secondary" :disabled="task.busy" @click="review(claim.receiptId)">{{ $t('imports.reviewLeftover') }}</AppButton></li>
        </ul>
        <div v-if="claimAfter || claimNext" class="app-actions import-actions">
          <AppButton variant="quiet" :disabled="!claimAfter || task.busy" @click="claimAfter = ''; scan()">{{ $t('imports.first') }}</AppButton>
          <AppButton variant="quiet" :disabled="!claimNext || task.busy" @click="claimAfter = claimNext; scan()">{{ $t('imports.next') }}</AppButton>
        </div>
        <section v-if="proof" class="import-confirmation" aria-live="polite">
          <h3>{{ proof.name }}</h3>
          <p>{{ $t(!proof.inputPresent ? 'imports.inputMissing' : proof.canRemove ? 'imports.duplicate' : 'imports.uniqueInput') }}</p>
          <p v-if="proof.warnings?.length">{{ $t('imports.retainedWarning') }}</p>
          <div class="app-actions import-actions">
            <AppButton variant="danger" :disabled="!proof.canRemove || task.busy" @click="resolve('confirm')">{{ $t('imports.removeDuplicate') }}</AppButton>
            <AppButton variant="secondary" :disabled="task.busy" @click="resolve('release')">{{ $t('imports.releaseClaim') }}</AppButton>
            <AppButton variant="quiet" :disabled="task.busy" @click="task.run(abandon)">{{ $t('imports.cancel') }}</AppButton>
          </div>
        </section>
      </section>
    </div>
  </section>
</template>
