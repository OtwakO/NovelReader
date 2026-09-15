<script lang="ts">
import { defineComponent } from 'vue';
import AppButton from '../../ui/components/AppButton.vue';
import { cancelTXTInboxReview, listTXTInboxClaims, resolveTXTInbox, reviewTXTInbox, scanTXTInbox, type TXTInboxClaim, type TXTInboxEntry, type TXTInboxReview } from '../../api/txt-imports';
import { ApiError } from '../../api/transport';
import { useImportQueue } from './import-queue';
import { createImportTask } from './import-task';
import { importErrorKey } from './import-feedback';
export default defineComponent({
  components: { AppButton },
  data: () => ({
    queue: useImportQueue(), task: createImportTask(), entries: [] as TXTInboxEntry[], claims: [] as TXTInboxClaim[],
    directory: '', after: '', next: '', claimAfter: '', claimNext: '', selected: [] as string[], proof: undefined as TXTInboxReview | undefined,
  }),
  watch: { 'queue.revision'() { this.scan(); } },
  mounted() { this.scan(); },
  beforeUnmount() { this.task.cancel(); },
  methods: {
    importErrorKey,
    async load(signal: AbortSignal) {
      const [scan, pending] = await Promise.all([scanTXTInbox(this.after, signal), listTXTInboxClaims(this.claimAfter, signal)]);
      signal.throwIfAborted();
      this.entries = scan.items; this.directory = scan.directory; this.next = scan.nextCursor || '';
      this.claims = pending.items; this.claimNext = pending.nextCursor || '';
      this.selected = this.selected.filter(name => scan.items.some(item => item.name === name && !item.problem && !item.receiptId));
    },
    scan() { void this.task.run(this.load); },
    async abandon(signal: AbortSignal) {
      const token = this.proof?.token; this.proof = undefined;
      if (token) {
        try { await cancelTXTInboxReview(token, signal); }
        catch (cause) { if (!(cause instanceof ApiError && cause.code === 'txt_inbox_review_expired')) throw cause; }
      }
    },
    review(id: string) { void this.task.run(async signal => { await this.abandon(signal); this.proof = await reviewTXTInbox(id, signal); }); },
    resolve(action: 'confirm' | 'release') {
      const token = this.proof?.token;
      if (!token) return;
      void this.task.run(async signal => {
        this.proof = undefined; // One attempt consumes approval, even when its response is lost.
        await resolveTXTInbox(token, action, signal); await this.load(signal);
      });
    },
  },
});
</script>

<template>
  <section class="import-section" aria-labelledby="inbox-title">
    <h2 id="inbox-title">{{ $t('imports.inbox') }}</h2>
    <p>{{ $t('imports.inboxHint') }}</p>
    <p v-if="directory"><code>DATA_DIR/{{ directory }}</code></p>
    <div class="app-actions import-actions">
      <AppButton variant="secondary" :busy="task.busy" @click="scan">{{ $t('imports.scan') }}</AppButton>
      <AppButton :disabled="!selected.length || task.busy" @click="queue.enqueue(selected); selected = []">{{ $t('imports.acquireSelected', { count: selected.length }) }}</AppButton>
    </div>
    <p v-if="task.error" role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
    <p v-if="!entries.length && !task.busy">{{ $t('imports.inboxEmpty') }}</p>
    <ul class="import-list">
      <li v-for="item in entries" :key="item.name">
        <label class="import-copy"><span><input v-model="selected" type="checkbox" :value="item.name" :disabled="!!item.problem || !!item.receiptId || task.busy"> {{ item.name }}</span>
          <small v-if="item.problem">{{ $t('imports.unavailableFile') }} ({{ item.problem }})</small>
          <small v-else-if="item.receiptId">{{ $t('imports.claimRequired') }}</small>
        </label>
      </li>
    </ul>
    <div class="app-actions import-actions">
      <AppButton variant="quiet" :disabled="!after || task.busy" @click="after = ''; selected = []; scan()">{{ $t('imports.first') }}</AppButton>
      <AppButton variant="quiet" :disabled="!next || task.busy" @click="after = next; selected = []; scan()">{{ $t('imports.next') }}</AppButton>
    </div>
    <h3>{{ $t('imports.leftovers') }}</h3>
    <p>{{ $t('imports.leftoversHint') }}</p>
    <p v-if="!claims.length && !task.busy">{{ $t('imports.noClaims') }}</p>
    <ul class="import-list">
      <li v-for="claim in claims" :key="claim.receiptId"><strong class="import-copy">{{ claim.name }}</strong><AppButton variant="secondary" :disabled="task.busy" @click="review(claim.receiptId)">{{ $t('imports.reviewLeftover') }}</AppButton></li>
    </ul>
    <div class="app-actions import-actions">
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
</template>
