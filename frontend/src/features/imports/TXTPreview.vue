<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import { getTXTPreviewSection, getTXTReparseSection, previewTXT, previewTXTReparse, type TXTPreview, type TXTPreviewSection } from '../../api/txt-imports';
import { ApiError } from '../../api/transport';
import AppButton from '../../ui/components/AppButton.vue';
import BookPreview from './BookPreview.vue';
import { createImportTask } from './import-task';
import { importErrorKey } from './import-feedback';

export default defineComponent({
  components: { AppButton, BookPreview },
  props: {
    sourceId: { type: String, required: true }, reparse: Boolean,
    preview: { type: Object as PropType<TXTPreview>, required: true },
    pattern: { type: String, default: '' }, busy: Boolean, compact: Boolean,
  },
  data: () => ({
    task: createImportTask(), page: undefined as TXTPreview | undefined, pageStart: 0,
    selected: 0, section: undefined as TXTPreviewSection | undefined,
  }),
  computed: {
    identity(): string { return `${this.sourceId}:${this.reparse}:${this.preview.analysisVersion}`; },
    entries() { return (this.page?.headings || []).map(heading => ({ key: String(heading.index), label: heading.title })); },
  },
  watch: { identity: { immediate: true, handler() {
    this.task.cancel(); this.page = this.preview; this.pageStart = 0; this.open(0);
  } } },
  beforeUnmount() { this.task.cancel(); },
  methods: {
    importErrorKey,
    open(index: number) {
      this.task.cancel(); this.selected = index; this.section = undefined;
      if (!this.preview.totalSections) return;
      void this.task.run(async signal => {
        const start = Math.floor(index / 25) * 25;
        const readPage = this.reparse ? previewTXTReparse : previewTXT;
        const readSection = this.reparse ? getTXTReparseSection : getTXTPreviewSection;
        const generation = this.preview.analysisVersion;
        const [page, section] = await Promise.all([
          start === this.pageStart ? Promise.resolve(this.page!) : readPage(this.sourceId, generation, start, signal),
          readSection(this.sourceId, generation, index, signal),
        ]);
        signal.throwIfAborted();
        if (page.analysisVersion !== generation || section.generation !== generation || section.index !== index) throw new ApiError(409, { code: 'txt_state_changed' });
        this.page = page; this.pageStart = start; this.section = section;
        await this.$nextTick(); signal.throwIfAborted();
        (this.$refs.layout as InstanceType<typeof BookPreview>).resetScroll();
      });
    },
    choose(key: string) { this.open(Number(key)); },
  },
});
</script>

<template>
  <section class="import-section" aria-labelledby="preview-title">
    <component :is="compact ? 'h3' : 'h2'" id="preview-title">{{ $t('imports.sectionPreview.title') }}</component>
    <p v-if="compact">{{ $t('imports.flow.chapterCount', { count: preview.totalSections }) }}</p>
    <template v-else>
      <p>{{ $t('imports.previewSummary', { count: preview.totalSections, encoding: preview.encoding }) }}</p>
      <p>{{ $t('imports.preset') }}: {{ $t(`imports.presets.${preview.preset}`) }}</p>
      <p v-if="pattern">{{ $t('imports.savedPattern') }}: <code>{{ pattern }}</code></p>
      <p v-for="reason in preview.reviewReasons || []" :key="reason">{{ $t(`imports.reasons.${reason}`) }}</p>
    </template>
    <BookPreview ref="layout" :entries="entries" :selected="String(selected)" :section="selected" :total-sections="preview.totalSections" :disabled="busy" :contents-label="$t('imports.sectionPreview.contents')" @select="choose" @previous="open(selected - 1)" @next="open(selected + 1)">
      <template #contents-footer>
        <div v-if="pageStart > 0 || page?.hasMore" class="contents-pages">
          <AppButton variant="secondary" :disabled="busy || pageStart === 0" @click="open(Math.max(0, pageStart - 25))">{{ $t('imports.sectionPreview.previousPage') }}</AppButton>
          <AppButton variant="secondary" :disabled="busy || !page?.hasMore" @click="open(pageStart + 25)">{{ $t('imports.sectionPreview.nextPage') }}</AppButton>
        </div>
      </template>
      <p v-if="task.busy" role="status">{{ $t('imports.sectionPreview.loading') }}</p>
      <div v-if="task.error"><p role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p><AppButton variant="secondary" @click="open(selected)">{{ $t('imports.refresh') }}</AppButton></div>
      <pre v-if="section" class="preview-text">{{ section.text }}</pre>
      <template #selection-action><div v-if="section && $slots['heading-action']" class="preview-resume"><span>{{ section.title }}</span><slot name="heading-action" :heading="section" /></div></template>
    </BookPreview>
    <p class="import-note">{{ $t(reparse ? 'imports.sectionPreview.reparseHint' : 'imports.sectionPreview.hint') }}</p>
  </section>
</template>

<style scoped>
.contents-pages { display: flex; flex-wrap: wrap; gap: var(--space-2); margin-top: var(--space-3); }
.contents-pages .app-button { flex: 1; font-size: var(--text-small); }
.preview-resume { display: flex; align-items: center; flex-wrap: wrap; gap: var(--space-3); margin-top: var(--space-4); }
.preview-resume > span { overflow-wrap: anywhere; }
</style>
