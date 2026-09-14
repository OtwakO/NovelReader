<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import AppButton from '../../ui/components/AppButton.vue';
import { acceptTXT, analyzeTXT, discardTXT, getTXTReceipt, previewTXT, type TXTEncoding, type TXTPreset, type TXTPreview, type TXTReceipt } from '../../api/txt-imports';
import { deleteBook } from '../../api/books';
import { importedTitle, importErrorKey } from './import-feedback';
import { createImportTask } from './import-task';
import './imports.css';
export default defineComponent({
  components: { RouterLink, AppButton },
  data: () => ({
    task: createImportTask(), receipt: undefined as TXTReceipt | undefined, preview: undefined as TXTPreview | undefined,
    name: '', author: '', encoding: '' as TXTEncoding, preset: '' as TXTPreset, pattern: '', start: 0, warnings: [] as string[],
    confirmDiscard: false, removed: false, cleanupPending: false, timer: undefined as ReturnType<typeof setInterval> | undefined,
    encodings: ['', 'utf-8', 'utf-16le', 'utf-16be', 'gb18030', 'big5'] as TXTEncoding[],
    presets: ['', 'chinese-chapters', 'english-chapters', 'generated-sections', 'custom'] as TXTPreset[],
  }),
  computed: {
    id(): string { return String(this.$route.params.id); },
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
    importErrorKey,
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
    }); },
  },
});
</script>

<template>
  <div class="imports-page">
    <RouterLink :to="{ path: '/imports', query: $route.query }">{{ $t('imports.back') }}</RouterLink>
    <h1>{{ receipt?.originalName || $t('imports.review') }}</h1>
    <p v-if="task.error && !(patternError && canAnalyze && preset === 'custom')" role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
    <template v-if="removed">
      <p role="status">{{ $t(cleanupPending ? (receipt?.libraryId ? 'imports.libraryCleanupPending' : 'imports.cleanupPending') : 'imports.discarded') }}</p>
      <AppButton v-if="cleanupPending" :busy="task.busy" @click="discard">{{ $t('imports.retryCleanup') }}</AppButton>
    </template>
    <template v-else>
      <div class="import-actions"><span v-if="receipt" role="status">{{ $t(`imports.state.${receipt.state}`) }}</span><AppButton variant="secondary" :busy="task.busy" @click="refresh">{{ $t('imports.refresh') }}</AppButton></div>
      <p v-if="warnings.length" role="status">{{ $t('imports.retainedWarning') }}</p>
      <RouterLink v-if="receipt?.libraryId" :to="`/books/${receipt.libraryId}`">{{ $t('imports.openBook') }}</RouterLink>
      <p v-if="receipt?.state === 'failed'">{{ $t('imports.failedHint') }}</p>
      <p v-if="receipt?.state === 'analysis_failed'">{{ $t('imports.analysisHint') }}</p>
      <section v-if="canAnalyze" class="import-section" aria-labelledby="interpretation-title">
        <h2 id="interpretation-title">{{ $t('imports.interpretation') }}</h2>
        <div class="import-actions">
          <label>{{ $t('imports.encoding') }}<select v-model="encoding" :disabled="task.busy"><option v-for="value in encodings" :key="value" :value="value">{{ value || $t('imports.automatic') }}</option></select></label>
          <label>{{ $t('imports.preset') }}<select v-model="preset" :disabled="task.busy"><option v-for="value in presets" :key="value" :value="value">{{ value ? $t(`imports.presets.${value}`) : $t('imports.automatic') }}</option></select></label>
        </div>
        <div v-if="preset === 'custom'" class="import-pattern">
          <label for="heading-pattern">{{ $t('imports.pattern') }}</label>
          <textarea id="heading-pattern" v-model="pattern" required rows="2" maxlength="2048" spellcheck="false" autocapitalize="off" :disabled="task.busy" :aria-invalid="patternError || undefined" :aria-describedby="patternError ? 'pattern-help pattern-error' : 'pattern-help'" @input="clearPatternError" />
          <p id="pattern-help">{{ $t('imports.patternHint') }}<br>{{ $t('imports.patternExample') }} <code>(?i)part [0-9]+.*</code></p>
          <p v-if="patternError" id="pattern-error" role="alert" class="import-error">{{ $t('imports.errors.pattern') }}</p>
        </div>
        <div class="import-actions"><AppButton variant="secondary" :busy="task.busy" :disabled="preset === 'custom' && !pattern" @click="analyze">{{ $t('imports.analyze') }}</AppButton></div>
        <p v-if="optionsChanged">{{ $t('imports.unappliedOptions') }}</p>
      </section>
      <section v-if="preview" class="import-section" aria-labelledby="preview-title">
        <h2 id="preview-title">{{ $t('imports.preview') }}</h2>
        <p>{{ $t('imports.previewSummary', { count: preview.totalSections, encoding: preview.encoding }) }}</p>
        <p>{{ $t('imports.preset') }}: {{ $t(`imports.presets.${preview.preset}`) }}</p>
        <p v-if="receipt?.pattern && preview.analysisVersion === receipt.analysisVersion">{{ $t('imports.savedPattern') }}: <code>{{ receipt.pattern }}</code></p>
        <p v-for="reason in preview.reviewReasons || []" :key="reason">{{ $t(`imports.reasons.${reason}`) }}</p>
        <div class="import-preview">
          <div>
<h3>{{ $t('imports.headings') }}</h3><ol :start="start + 1"><li v-for="heading in preview.headings" :key="heading.index">{{ heading.title }} <small v-if="heading.generated">({{ $t('imports.generated') }})</small></li></ol>
            <div class="import-actions"><AppButton variant="secondary" :disabled="start === 0 || task.busy" @click="previewPage(Math.max(0, start - 25))">{{ $t('imports.previous') }}</AppButton><AppButton variant="secondary" :disabled="!preview.hasMore || task.busy" @click="previewPage(start + 25)">{{ $t('imports.next') }}</AppButton></div>
          </div>
          <div><h3>{{ $t('imports.sample') }}</h3><pre class="import-sample">{{ preview.sample }}</pre><p v-if="preview.sampleTruncated">{{ $t('imports.truncated') }}</p></div>
        </div>
      </section>
      <form v-if="receipt && ['ready', 'needs_review'].includes(receipt.state)" class="import-section" @submit.prevent="accept">
        <h2>{{ $t('imports.addToLibrary') }}</h2>
        <label>{{ $t('imports.bookTitle') }}<input v-model="name" required maxlength="256" :disabled="task.busy"></label>
        <label>{{ $t('imports.author') }}<input v-model="author" maxlength="128" :disabled="task.busy"></label>
        <AppButton type="submit" :busy="task.busy" :disabled="!canAccept">{{ $t('imports.confirmAdd') }}</AppButton>
      </form>
      <section v-if="receipt && !receipt.libraryId" class="import-section">
        <AppButton variant="quiet" :disabled="task.busy" @click="confirmDiscard = true">{{ $t('imports.discard') }}</AppButton>
        <div v-if="confirmDiscard" class="import-confirmation">
          <p>{{ $t('imports.discardConfirm', { name: receipt.originalName }) }}</p>
          <AppButton variant="danger" :busy="task.busy" @click="discard">{{ $t('imports.confirmDiscard') }}</AppButton><AppButton variant="quiet" :disabled="task.busy" @click="confirmDiscard = false">{{ $t('imports.cancel') }}</AppButton>
        </div>
      </section>
    </template>
  </div>
</template>
