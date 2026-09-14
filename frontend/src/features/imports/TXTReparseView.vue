<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import AppButton from '../../ui/components/AppButton.vue';
import { ApiError } from '../../api/transport';
import { applyTXTReparse, discardTXTReparse, getTXTReparse, impactTXTReparse, prepareTXTReparse, previewTXTReparse, type TXTEncoding, type TXTPreset, type TXTOptions, type TXTPreview as Preview, type TXTReparseStatus, type TXTReparseImpact } from '../../api/txt-imports';
import { invalidateReadingState, waitForProgressWrites } from '../reader/progress-writer';
import TXTInterpretationOptions from './TXTInterpretationOptions.vue';
import TXTPreview from './TXTPreview.vue';
import { createImportTask } from './import-task';
import { importErrorKey } from './import-feedback';
import './imports.css';

export default defineComponent({
  components: { RouterLink, AppButton, TXTInterpretationOptions, TXTPreview },
  data: () => ({
    task: createImportTask(), status: undefined as TXTReparseStatus | undefined,
    preview: undefined as Preview | undefined, impact: undefined as TXTReparseImpact | undefined,
    encoding: '' as TXTEncoding, preset: '' as TXTPreset, pattern: '', start: 0,
    resume: undefined as { index: number; title: string } | undefined,
    mustRefresh: false, confirmApply: false, confirmDiscard: false, warnings: [] as string[],
    applyAttempt: undefined as number | undefined, timer: undefined as ReturnType<typeof setInterval> | undefined,
  }),
  computed: {
    id(): string { return String(this.$route.params.bookId); },
    options(): TXTOptions { return { encoding: this.encoding, preset: this.preset, pattern: this.preset === 'custom' ? this.pattern : '' }; },
    optionsChanged(): boolean {
      const saved = this.status?.candidate?.options;
      return !saved || saved.encoding !== this.encoding || saved.preset !== this.preset || (saved.pattern || '') !== this.options.pattern;
    },
    processing(): boolean { return ['queued', 'analyzing'].includes(this.status?.candidate?.state || ''); },
    patternError(): boolean { return !!this.task.error && importErrorKey(this.task.error) === 'imports.errors.pattern'; },
    canApply(): boolean { return !this.mustRefresh && !this.optionsChanged && !!this.impact && this.preview?.analysisVersion === this.impact.generation && (!!this.impact.resume || !!this.resume); },
    applied(): boolean { return !!this.applyAttempt && this.status?.activeGeneration === this.applyAttempt; },
  },
  watch: { id: { immediate: true, handler() {
    this.task.cancel(); this.status = undefined; this.preview = undefined; this.impact = undefined; this.resume = undefined;
    this.mustRefresh = false; this.applyAttempt = undefined; this.confirmApply = false; this.confirmDiscard = false; this.start = 0; this.warnings = [];
    this.refresh();
  } } },
  mounted() { this.timer = setInterval(() => { if (this.processing && !this.mustRefresh && document.visibilityState === 'visible') this.refresh(); }, 5000); },
  beforeUnmount() { clearInterval(this.timer); this.task.cancel(); },
  methods: {
    importErrorKey,
    useSavedOptions() {
      const options = this.status?.candidate?.options || this.status?.activeOptions;
      if (options) { this.encoding = options.encoding; this.preset = options.preset; this.pattern = options.pattern || ''; }
      this.confirmApply = false;
    },
    edited() { this.confirmApply = false; if (this.patternError) this.task.error = undefined; },
    async load(signal: AbortSignal) {
      this.mustRefresh = true; this.confirmApply = false; this.confirmDiscard = false;
      await waitForProgressWrites(this.id); signal.throwIfAborted();
      const status = await getTXTReparse(this.id, signal); signal.throwIfAborted();
      const first = !this.status, previous = this.status?.candidate?.generation;
      this.status = status;
      if (this.applied) invalidateReadingState(this.id);
      if (first) this.useSavedOptions();
      if (previous !== status.candidate?.generation) { this.preview = undefined; this.impact = undefined; this.start = 0; this.resume = undefined; }
      const candidate = status.candidate;
      if (candidate && ['ready', 'needs_review'].includes(candidate.state)) {
        const [preview, impact] = await Promise.all([
          this.preview ? Promise.resolve(this.preview) : previewTXTReparse(this.id, candidate.generation, this.start, signal),
          impactTXTReparse(this.id, candidate.generation, signal),
        ]);
        signal.throwIfAborted();
        if (impact.activeGeneration !== status.activeGeneration || impact.contentRevision !== status.contentRevision || preview.analysisVersion !== impact.generation) throw new ApiError(409, { code: 'txt_state_changed' });
        this.preview = preview; this.impact = impact;
      } else { this.preview = undefined; this.impact = undefined; }
      this.mustRefresh = false;
    },
    refresh() { void this.task.run(this.load); },
    mutate(action: (signal: AbortSignal) => Promise<void>) {
      this.confirmApply = false; this.confirmDiscard = false;
      void this.task.run(async signal => {
        this.mustRefresh = true;
        try { await action(signal); signal.throwIfAborted(); await this.load(signal); }
        catch (cause) {
          // Validation failures did not change server state; uncertain outcomes and
          // conflicts require status recovery, never automatic mutation retries.
          if (!signal.aborted && cause instanceof ApiError && cause.status === 400) this.mustRefresh = false;
          throw cause;
        }
      });
    },
    prepare() {
      if (!this.status || this.mustRefresh) return;
      this.mutate(async signal => {
        this.applyAttempt = undefined;
        const result = await prepareTXTReparse(this.id, this.status!.contentRevision, this.status!.candidate?.generation || 0, this.options, signal);
        signal.throwIfAborted(); this.warnings = result.warnings || [];
      });
    },
    discard() {
      if (!this.status?.candidate || this.mustRefresh) return;
      this.mutate(signal => discardTXTReparse(this.id, this.status!.contentRevision, this.status!.candidate!.generation, signal));
    },
    apply() {
      if (!this.canApply || !this.confirmApply) return;
      const impact = this.impact!;
      this.mutate(async signal => {
        this.applyAttempt = impact.generation;
        await applyTXTReparse(this.id, { generation: impact.generation, activeGeneration: impact.activeGeneration, contentRevision: impact.contentRevision, stateVersion: impact.stateVersion, resumeChapter: this.resume?.index }, signal);
        signal.throwIfAborted(); invalidateReadingState(this.id);
      });
    },
    page(offset: number) {
      if (this.mustRefresh || !this.status?.candidate) return;
      void this.task.run(async signal => {
        this.mustRefresh = true; this.confirmApply = false;
        const preview = await previewTXTReparse(this.id, this.status!.candidate!.generation, offset, signal);
        signal.throwIfAborted(); this.preview = preview; this.start = offset; this.mustRefresh = false;
      });
    },
    chooseResume(heading: { index: number; title: string }) { this.resume = heading; this.confirmApply = false; },
  },
});
</script>

<template>
  <div class="imports-page">
    <RouterLink :to="`/books/${encodeURIComponent(id)}`">{{ $t('reader.back') }}</RouterLink>
    <h1>{{ $t('imports.reparse.title') }}</h1>
    <p>{{ $t('imports.reparse.intro') }}</p>
    <p v-if="task.error && !patternError" role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
    <p v-if="mustRefresh && !task.busy" role="status">{{ $t('imports.reparse.refreshRequired') }}</p>
    <div class="import-actions"><AppButton variant="secondary" :busy="task.busy" @click="refresh">{{ $t('imports.refresh') }}</AppButton><RouterLink v-if="status" :to="{ name: 'reader', params: { bookId: id } }">{{ $t('imports.reparse.readCurrent') }}</RouterLink></div>
    <p v-if="applied" role="status">{{ $t('imports.reparse.applied') }}</p>
    <p v-if="warnings.length" role="status">{{ $t('imports.reparse.analysisPending') }}</p>
    <template v-if="status">
      <p class="import-copy"><strong>{{ status.name }}</strong></p>
      <p>{{ $t('imports.reparse.active') }}: {{ status.activeOptions.encoding || $t('imports.automatic') }} · {{ status.activeOptions.preset ? $t(`imports.presets.${status.activeOptions.preset}`) : $t('imports.automatic') }}</p>
      <p v-if="status.activeOptions.pattern"><code>{{ status.activeOptions.pattern }}</code></p>
      <section class="import-section" aria-labelledby="interpretation-title">
        <h2 id="interpretation-title">{{ $t('imports.interpretation') }}</h2>
        <TXTInterpretationOptions v-model:encoding="encoding" v-model:preset="preset" v-model:pattern="pattern" :busy="task.busy" :pattern-error="patternError" @update:encoding="edited" @update:preset="edited" @update:pattern="edited" />
        <div class="import-actions"><AppButton :busy="task.busy" :disabled="mustRefresh || (preset === 'custom' && !pattern)" @click="prepare">{{ $t(status.candidate ? 'imports.reparse.replace' : 'imports.reparse.prepare') }}</AppButton><AppButton variant="quiet" :disabled="task.busy" @click="useSavedOptions">{{ $t('imports.reparse.savedOptions') }}</AppButton></div>
        <p v-if="status.candidate" role="status">{{ $t(status.candidate.state === 'ready' ? 'imports.reparse.ready' : `imports.state.${status.candidate.state === 'queued' ? 'received' : status.candidate.state}`) }}</p>
        <p v-if="status.candidate?.state === 'analysis_failed'">{{ $t('imports.analysisHint') }}</p>
        <p v-if="status.candidate && optionsChanged">{{ $t('imports.reparse.draft') }}</p>
      </section>
      <TXTPreview v-if="preview" :preview="preview" :pattern="status.candidate?.options.pattern" :start="start" :busy="task.busy || mustRefresh" @page="page">
        <template #heading-action="{ heading }"><AppButton v-if="impact" variant="quiet" :disabled="task.busy || mustRefresh" :aria-pressed="resume?.index === heading.index" @click="chooseResume(heading)">{{ $t('imports.reparse.resumeHere') }}</AppButton></template>
      </TXTPreview>
      <section v-if="impact" class="import-section" aria-labelledby="impact-title">
        <h2 id="impact-title">{{ $t('imports.reparse.impact') }}</h2>
        <p>{{ $t('imports.reparse.bookmarks', { kept: impact.preservedBookmarks, unresolved: impact.unresolvedBookmarks }) }}</p>
        <p>{{ $t('imports.reparse.orphans') }}</p>
        <p v-if="resume">{{ resume.index === 0 ? $t('imports.reparse.fromBeginning') : $t('imports.reparse.selected', { title: resume.title }) }}</p>
        <p v-else-if="impact.resume">{{ $t('imports.reparse.preserved', { title: impact.resume.chapterTitle }) }}</p>
        <p v-else>{{ $t('imports.reparse.chooseResume') }}</p>
        <div class="import-actions"><AppButton variant="secondary" :disabled="task.busy || mustRefresh" @click="chooseResume({index:0,title:''})">{{ $t('imports.reparse.beginning') }}</AppButton><AppButton v-if="resume && impact.resume" variant="quiet" :disabled="task.busy" @click="resume=undefined; confirmApply=false">{{ $t('imports.reparse.usePreserved') }}</AppButton></div>
        <AppButton :disabled="task.busy || !canApply" @click="confirmApply=true">{{ $t('imports.reparse.apply') }}</AppButton>
        <div v-if="confirmApply" class="import-confirmation"><p>{{ $t('imports.reparse.confirm') }}</p><AppButton :busy="task.busy" :disabled="!canApply" @click="apply">{{ $t('imports.reparse.confirmApply') }}</AppButton><AppButton variant="quiet" :disabled="task.busy" @click="confirmApply=false">{{ $t('imports.cancel') }}</AppButton></div>
      </section>
      <section v-if="status.candidate" class="import-section">
        <AppButton variant="quiet" :disabled="task.busy || mustRefresh" @click="confirmDiscard=true">{{ $t('imports.reparse.discard') }}</AppButton>
        <div v-if="confirmDiscard" class="import-confirmation"><p>{{ $t('imports.reparse.discardHint') }}</p><AppButton :busy="task.busy" @click="discard">{{ $t('imports.reparse.confirmDiscard') }}</AppButton><AppButton variant="quiet" @click="confirmDiscard=false">{{ $t('imports.cancel') }}</AppButton></div>
      </section>
    </template>
  </div>
</template>
