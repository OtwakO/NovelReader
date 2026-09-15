<script lang="ts">
import AppDisclosure from '../../ui/components/AppDisclosure.vue';
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import AppButton from '../../ui/components/AppButton.vue';
import TXTInterpretationOptions from './TXTInterpretationOptions.vue';
import TXTPreview from './TXTPreview.vue';
import { acceptTXT, analyzeTXT, discardTXT, getTXTReceipt, previewTXT, type TXTEncoding, type TXTPreset, type TXTPreview as Preview, type TXTReceipt } from '../../api/txt-imports';
import { deleteBook } from '../../api/books';
import { importedTitle, importErrorKey, analysisErrorKey } from './import-feedback';
import { createImportTask } from './import-task';
import './imports.css';
export default defineComponent({
  components: { AppDisclosure, RouterLink, AppButton, TXTInterpretationOptions, TXTPreview },
  props: { receiptId: { type: String, required: true } },
  emits: ['updated', 'removed', 'close'],
  data: () => ({
    task: createImportTask(), receipt: undefined as TXTReceipt | undefined, preview: undefined as Preview | undefined,
    name: '', author: '', encoding: '' as TXTEncoding, preset: '' as TXTPreset, pattern: '', start: 0, warnings: [] as string[],
    confirmDiscard: false, removed: false, cleanupPending: false, timer: undefined as ReturnType<typeof setInterval> | undefined,
  }),
  computed: {
    id(): string { return this.receiptId; },
    canAnalyze(): boolean { return !!this.receipt && ['received', 'ready', 'needs_review', 'analysis_failed'].includes(this.receipt.state); },
    requestedPattern(): string { return this.preset === 'custom' ? this.pattern : ''; },
    patternError(): boolean { return !!this.task.error && importErrorKey(this.task.error) === 'imports.errors.pattern'; },
    optionsChanged(): boolean { return !!this.receipt && (this.encoding !== this.receipt.encoding || this.preset !== this.receipt.preset || this.requestedPattern !== (this.receipt.pattern || '')); },
    canAccept(): boolean { return !!this.receipt && ['ready', 'needs_review'].includes(this.receipt.state) && this.preview?.analysisVersion === this.receipt.analysisVersion && !this.optionsChanged && !!this.name.trim(); },
  },
  watch: { id: { immediate: true, handler() {
    this.task.cancel(); this.receipt = undefined; this.preview = undefined; this.name = ''; this.author = ''; this.start = 0;
    this.warnings = []; this.confirmDiscard = false; this.removed = false; this.cleanupPending = false; this.refresh();
  } } },
  mounted() {
    this.timer = setInterval(() => {
      if (!this.removed && document.visibilityState === 'visible' && this.receipt && ['receiving', 'received', 'analyzing'].includes(this.receipt.state)) this.refresh();
    }, 5000);
  },
  beforeUnmount() { clearInterval(this.timer); this.task.cancel(); },
  methods: {
    importErrorKey, analysisErrorKey,
    clearPatternError() { if (this.patternError) this.task.error = undefined; },
    async load(signal: AbortSignal) {
      const value = await getTXTReceipt(this.id, signal);
      signal.throwIfAborted();
      if (!this.receipt) { this.name = importedTitle(value.originalName); this.encoding = value.encoding; this.preset = value.preset; this.pattern = value.pattern || ''; }
      const previousVersion = this.receipt?.analysisVersion; this.receipt = value;
      if (value.state === 'removing') { this.removed = true; this.cleanupPending = true; }
      if (['ready', 'needs_review', 'published'].includes(value.state)) {
        if (previousVersion !== value.analysisVersion) { this.start = 0; this.preview = undefined; }
        if (!this.preview) { const preview = await previewTXT(value.id, value.analysisVersion, this.start, signal); signal.throwIfAborted(); this.preview = preview; }
      } else this.preview = undefined;
      this.$emit('updated', value);
    },
    refresh() { void this.task.run(this.load); },
    previewPage(offset: number) { void this.task.run(async signal => {
      const value = await previewTXT(this.id, this.receipt!.analysisVersion, offset, signal);
      signal.throwIfAborted(); this.start = offset; this.preview = value;
    }); },
    analyze() { void this.task.run(async signal => {
      const result = await analyzeTXT(this.id, this.receipt!.analysisVersion, { encoding: this.encoding, preset: this.preset, pattern: this.requestedPattern }, signal);
      signal.throwIfAborted(); this.preview = undefined; this.warnings = result.warnings || []; await this.load(signal);
    }); },
    accept() {
      if (!this.canAccept) return;
      void this.task.run(async signal => {
        await acceptTXT(this.id, this.preview!.analysisVersion, this.name.trim(), this.author.trim(), signal); await this.load(signal);
      });
    },
    discard() { void this.task.run(async signal => {
      // A retained publication-removal record still belongs to the library route;
      // pending-only discard must never bypass that provider ownership boundary.
      const result = this.receipt?.state === 'removing' && this.receipt.libraryId
        ? await deleteBook(this.receipt.libraryId, AbortSignal.any([signal, AbortSignal.timeout(30_000)]))
        : await discardTXT(this.id, signal);
      signal.throwIfAborted(); this.removed = true; this.confirmDiscard = false;
      this.cleanupPending = !!result.warnings?.length; this.warnings = result.warnings || [];
      if (!this.cleanupPending) this.$emit('removed', this.id);
    }); },
  },
});
</script>

<template>
  <section class="import-editor" aria-labelledby="import-review-title">
    <header class="import-editor-heading"><h2 id="import-review-title" tabindex="-1">{{ receipt?.originalName || $t('imports.review') }}</h2><div class="app-actions import-actions"><AppButton v-if="receipt && ['ready', 'needs_review'].includes(receipt.state)" type="submit" form="import-add-form" :busy="task.busy" :disabled="!canAccept">{{ $t('imports.confirmAdd') }}</AppButton><AppButton variant="quiet" :disabled="task.busy" @click="$emit('close')">{{ $t('imports.flow.closeReview') }}</AppButton></div></header>
    <p v-if="task.error && !(patternError && canAnalyze && preset === 'custom')" role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
    <template v-if="removed">
      <p role="status">{{ $t(cleanupPending ? (receipt?.libraryId ? 'imports.libraryCleanupPending' : 'imports.cleanupPending') : 'imports.discarded') }}</p>
      <AppButton v-if="cleanupPending" :busy="task.busy" @click="discard">{{ $t('imports.retryCleanup') }}</AppButton>
    </template>
    <template v-else>
      <div v-if="task.error || !receipt || ['receiving', 'received', 'analyzing'].includes(receipt.state)" class="import-review-status">
        <span v-if="receipt && ['receiving', 'received', 'analyzing'].includes(receipt.state)" role="status">{{ $t(`imports.state.${receipt.state}`) }}</span>
        <AppButton variant="secondary" :busy="task.busy" @click="refresh">{{ $t('imports.refresh') }}</AppButton>
      </div>
      <p v-if="warnings.length" role="status">{{ $t('imports.retainedWarning') }}</p>
      <RouterLink v-if="receipt?.libraryId" class="app-button app-button--secondary import-read" :to="{ name: 'reader', params: { bookId: receipt.libraryId } }">{{ $t('imports.flow.read') }}</RouterLink>
      <p v-if="receipt?.state === 'failed'">{{ $t('imports.failedHint') }}</p>
      <p v-if="receipt?.state === 'analysis_failed'">{{ $t(analysisErrorKey(receipt.errorCode)) }}</p>
      <p v-if="receipt?.state === 'needs_review'" class="import-review-guidance">{{ $t('imports.flow.reviewHint') }}</p>
      <AppDisclosure v-if="canAnalyze" class="import-options" :open="receipt?.state === 'analysis_failed'">
        <template #summary>{{ $t('imports.flow.adjustChapters') }}</template>
        <TXTInterpretationOptions v-model:encoding="encoding" v-model:preset="preset" v-model:pattern="pattern" :busy="task.busy" :pattern-error="patternError" @update:pattern="clearPatternError" />
        <div class="app-actions import-actions"><AppButton variant="secondary" :busy="task.busy" :disabled="preset === 'custom' && !pattern" @click="analyze">{{ $t('imports.analyze') }}</AppButton></div>
        <p v-if="optionsChanged">{{ $t('imports.unappliedOptions') }}</p>
      </AppDisclosure>
      <TXTPreview v-if="preview" compact :preview="preview" :start="start" :pattern="receipt?.pattern" :busy="task.busy" @page="previewPage" />
      <form v-if="receipt && ['ready', 'needs_review'].includes(receipt.state)" id="import-add-form" class="import-section" @submit.prevent="accept">
        <AppDisclosure class="import-options">
          <template #summary>{{ $t('imports.flow.bookDetails') }}</template>
        <label>{{ $t('imports.bookTitle') }}<input v-model="name" required maxlength="256" :disabled="task.busy"></label>
        <label>{{ $t('imports.author') }}<input v-model="author" maxlength="128" :disabled="task.busy"></label>
        </AppDisclosure>
      </form>
      <section v-if="receipt && !receipt.libraryId" class="import-section">
        <AppButton variant="quiet" :disabled="task.busy" @click="confirmDiscard = true">{{ $t('imports.discard') }}</AppButton>
        <div v-if="confirmDiscard" class="import-confirmation">
          <p>{{ $t('imports.discardConfirm', { name: receipt.originalName }) }}</p>
          <div class="app-actions">
            <AppButton variant="danger" :busy="task.busy" @click="discard">{{ $t('imports.confirmDiscard') }}</AppButton>
            <AppButton variant="quiet" :disabled="task.busy" @click="confirmDiscard = false">{{ $t('imports.cancel') }}</AppButton>
          </div>
        </div>
      </section>
    </template>
  </section>
</template>
