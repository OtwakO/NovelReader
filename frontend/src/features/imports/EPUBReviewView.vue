<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import AppButton from '../../ui/components/AppButton.vue';
import { acceptEPUB, discardEPUB, getEPUBReceipt, previewEPUB, retryEPUB, type EPUBReceipt, type EPUBPreview } from '../../api/epub-imports';
import { deleteBook } from '../../api/books';
import { analysisErrorKey, encoderNoticeKey, importedTitle, importErrorKey } from './import-feedback';
import { receiptState } from './import-format';
import { createImportTask } from './import-task';
import EPUBSectionPreview from './EPUBSectionPreview.vue';
import './imports.css';

export default defineComponent({
  components: { RouterLink, AppButton, EPUBSectionPreview },
  props: { receiptId: { type: String, required: true } },
  emits: ['updated', 'removed', 'close'],
  data: () => ({
    task: createImportTask(), receipt: undefined as EPUBReceipt | undefined, preview: undefined as EPUBPreview | undefined,
    name: '', author: '', warnings: [] as string[], confirmDiscard: false, removed: false, cleanupPending: false,
    timer: undefined as ReturnType<typeof setInterval> | undefined,
  }),
  computed: {
    preparing(): boolean { return !!this.receipt && ['queued', 'preparing', 'finalizing'].includes(this.receipt.preparationState || ''); },
    pending(): boolean { return !!this.receipt && (['receiving', 'finalizing'].includes(this.receipt.state) || this.preparing); },
    canRetry(): boolean { return this.receipt?.state === 'acquired' && !this.receipt.libraryId && (this.receipt.generation === 0 || this.receipt.preparationState === 'failed'); },
    canDiscard(): boolean { return !!this.receipt && !this.receipt.libraryId && !['preparing', 'finalizing'].includes(this.receipt.preparationState || ''); },
    canAccept(): boolean { return !!this.receipt && receiptState(this.receipt) === 'ready' && this.preview?.generation === this.receipt.generation && !!this.name.trim() && !this.task.error; },
    notices(): string[] { return this.preview?.notices || this.receipt?.notices || []; },
  },
  watch: { receiptId: { immediate: true, handler() {
    this.task.cancel(); this.receipt = undefined; this.preview = undefined; this.name = ''; this.author = '';
    this.warnings = []; this.confirmDiscard = false; this.removed = false; this.cleanupPending = false; this.refresh();
  } } },
  mounted() { this.timer = setInterval(() => { if (!this.removed && this.pending && document.visibilityState === 'visible') this.refresh(); }, 5000); },
  beforeUnmount() { clearInterval(this.timer); this.task.cancel(); },
  methods: {
    importErrorKey, analysisErrorKey, encoderNoticeKey,
    diagnosticKey(code: string): string { const key = `imports.epub.diagnostics.${code}`; return this.$te(key) ? key : 'imports.epub.contentChanged'; },
    async load(signal: AbortSignal) {
      const value = await getEPUBReceipt(this.receiptId, signal);
      signal.throwIfAborted();
      if (!this.receipt) this.name = importedTitle(value.originalName);
      if (this.receipt?.generation !== value.generation) { this.preview = undefined; }
      this.receipt = value;
      if (value.state === 'removing') { this.removed = true; this.cleanupPending = true; }
      if (value.state === 'acquired' && value.preparationState === 'ready') {
        if (!this.preview) {
          const preview = await previewEPUB(value.id, value.generation, 0, signal);
          signal.throwIfAborted(); this.preview = preview;
          this.name = preview.title.trim() || importedTitle(value.originalName); this.author = preview.authors.join(', ');
        }
      } else this.preview = undefined;
      this.$emit('updated', { ...value, notices: this.preview ? (this.preview.notices || []) : value.notices });
    },
    refresh() { void this.task.run(this.load); },
    retry() { void this.task.run(async signal => {
      const result = await retryEPUB(this.receiptId, this.receipt!.generation, signal);
      signal.throwIfAborted(); this.warnings = result.warnings || []; await this.load(signal);
    }); },
    accept() {
      if (!this.canAccept) return;
      void this.task.run(async signal => { await acceptEPUB(this.receiptId, this.preview!.generation, this.name.trim(), this.author.trim(), signal); await this.load(signal); });
    },
    discard() { void this.task.run(async signal => {
      const result = this.receipt?.libraryId
        ? await deleteBook(this.receipt.libraryId, AbortSignal.any([signal, AbortSignal.timeout(30_000)]))
        : await discardEPUB(this.receiptId, signal);
      signal.throwIfAborted(); this.removed = true; this.confirmDiscard = false;
      this.cleanupPending = !!result.warnings?.length; this.warnings = result.warnings || [];
      if (!this.cleanupPending) this.$emit('removed', this.receiptId);
    }); },
  },
});
</script>

<template>
  <section class="import-editor" aria-labelledby="import-review-title">
    <header class="import-editor-heading"><h2 id="import-review-title" tabindex="-1">{{ receipt?.originalName || $t('imports.review') }}</h2><AppButton variant="quiet" :disabled="task.busy" @click="$emit('close')">{{ $t('imports.flow.closeReview') }}</AppButton></header>
    <p v-if="task.error" role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
    <template v-if="removed">
      <p role="status">{{ $t(cleanupPending ? (receipt?.libraryId ? 'imports.libraryCleanupPending' : 'imports.cleanupPending') : 'imports.discarded') }}</p>
      <AppButton v-if="cleanupPending" :busy="task.busy" @click="discard">{{ $t('imports.retryCleanup') }}</AppButton>
    </template>
    <template v-else>
      <div v-if="task.error || !receipt || pending" class="import-review-status"><span v-if="pending" role="status">{{ $t('imports.epub.preparing') }}</span><AppButton variant="secondary" :busy="task.busy" @click="refresh">{{ $t('imports.refresh') }}</AppButton></div>
      <p v-for="notice in notices" :key="notice" class="import-note" role="status">{{ $t(encoderNoticeKey(notice)) }}</p>
      <p v-if="warnings.length" role="status">{{ $t('imports.epub.retainedWarning') }}</p>
      <RouterLink v-if="receipt?.libraryId" class="app-button app-button--secondary import-read" :to="{ name: 'reader', params: { bookId: receipt.libraryId } }">{{ $t('imports.flow.read') }}</RouterLink>
      <p v-if="receipt?.state === 'failed'">{{ $t('imports.epub.acquisitionFailed') }}</p>
      <p v-if="receipt?.preparationState === 'failed'" class="import-error">{{ $t(analysisErrorKey(receipt.errorCode)) }}</p>
      <p v-if="receipt?.state === 'acquired' && receipt.generation === 0">{{ $t('imports.epub.unprepared') }}</p>
      <AppButton v-if="canRetry" variant="secondary" :busy="task.busy" :disabled="!!task.error" @click="retry">{{ $t('imports.epub.retry') }}</AppButton>
      <p v-if="receipt" class="import-note">{{ $t(receipt.imageMode === 'optimized' ? 'imports.epub.optimizedImages' : 'imports.epub.originalImages') }}</p>
      <form v-if="preview && !receipt?.libraryId" id="epub-add-form" class="import-fields" @submit.prevent="accept">
        <label>{{ $t('imports.bookTitle') }}<input v-model="name" required maxlength="256" :disabled="task.busy"></label>
        <label>{{ $t('imports.author') }}<input v-model="author" maxlength="128" :disabled="task.busy"></label>
      </form>
      <section v-if="preview" class="import-section" aria-labelledby="epub-preview-title">
        <h3 id="epub-preview-title">{{ $t('imports.epub.previewTitle') }}</h3>
        <p>{{ $t('imports.epub.sectionCount', { count: preview.totalSections }) }}</p>
        <p v-if="preview.needsReview" class="import-review-guidance">{{ $t('imports.epub.reviewHint') }}</p>
        <ul v-if="preview.diagnostics.length"><li v-for="code in preview.diagnostics" :key="code">{{ $t(diagnosticKey(code)) }}</li></ul>
        <EPUBSectionPreview :receipt-id="receiptId" :generation="preview.generation" :total-sections="preview.totalSections" />
      </section>
      <section v-if="receipt && !receipt.libraryId" class="import-section">
        <div class="app-actions"><AppButton v-if="preview" type="submit" form="epub-add-form" :busy="task.busy" :disabled="!canAccept">{{ $t('imports.confirmAdd') }}</AppButton><AppButton variant="quiet" :disabled="task.busy || !canDiscard || !!task.error" @click="confirmDiscard = true">{{ $t('imports.discard') }}</AppButton></div>
        <p v-if="!canDiscard" class="import-note">{{ $t('imports.epub.waitToDiscard') }}</p>
        <div v-if="confirmDiscard" class="import-confirmation"><p>{{ $t('imports.epub.discardConfirm', { name: receipt.originalName }) }}</p><div class="app-actions"><AppButton variant="danger" :busy="task.busy" :disabled="!canDiscard" @click="discard">{{ $t('imports.confirmDiscard') }}</AppButton><AppButton variant="quiet" :disabled="task.busy" @click="confirmDiscard = false">{{ $t('imports.cancel') }}</AppButton></div></div>
      </section>
    </template>
  </section>
</template>
