<script lang="ts">
import { defineComponent } from 'vue';
import AppButton from '../../ui/components/AppButton.vue';
import ProseRenderer from '../reader/ProseRenderer.vue';
import { getEPUBPreviewNavigation, getEPUBPreviewSection, type EPUBPreviewEntry, type EPUBPreviewNavigation, type EPUBPreviewTarget } from '../../api/epub-review';
import type { StructuredProseDocument } from '../../api/structured-prose';
import { createImportTask } from './import-task';
import { importErrorKey } from './import-feedback';

export default defineComponent({
  components: { AppButton, ProseRenderer },
  props: {
    receiptId: { type: String, required: true },
    generation: { type: Number, required: true },
    totalSections: { type: Number, required: true },
  },
  data: () => ({
    task: createImportTask(), navigation: undefined as EPUBPreviewNavigation | undefined,
    document: undefined as StructuredProseDocument | undefined,
    target: { section: 0 } as EPUBPreviewTarget, selectedEntry: '',
  }),
  computed: {
    identity(): string { return `${this.receiptId}:${this.generation}`; },
    entries(): { label: string; target?: EPUBPreviewTarget }[] {
      const result: { label: string; target?: EPUBPreviewTarget }[] = [];
      const visit = (entries: EPUBPreviewEntry[], depth: number) => {
        for (const entry of entries) {
          const label = entry.label.trim() || (entry.target ? this.$t('imports.epub.sectionNumber', { number: entry.target.section + 1 }) : this.$t('imports.epub.untitledEntry'));
          result.push({ label: `${'　'.repeat(Math.min(depth, 6))}${label}`, target: entry.unavailable ? undefined : entry.target });
          visit(entry.children, depth + 1);
        }
      };
      visit(this.navigation?.entries || [], 0);
      return result;
    },
  },
  watch: { identity: { immediate: true, handler() {
    this.task.cancel(); this.navigation = undefined; this.document = undefined;
    this.target = { section: 0 }; this.selectedEntry = ''; this.load();
  } } },
  beforeUnmount() { this.task.cancel(); },
  methods: {
    importErrorKey,
    load() {
      void this.task.run(async signal => {
        if (!this.navigation) {
          const navigation = await getEPUBPreviewNavigation(this.receiptId, this.generation, this.totalSections, signal);
          signal.throwIfAborted(); this.navigation = navigation;
        }
        if (!this.totalSections) return;
        const document = await getEPUBPreviewSection(this.receiptId, this.generation, this.target.section, signal);
        signal.throwIfAborted(); this.document = document;
        await this.$nextTick(); signal.throwIfAborted();
        const panel = this.$refs.content as HTMLElement;
        const renderer = this.$refs.renderer as InstanceType<typeof ProseRenderer>;
        const anchor = this.target.anchor ? renderer.findAnchor(this.target.anchor) : undefined;
        panel.scrollTop = anchor ? panel.scrollTop + anchor.getBoundingClientRect().top - panel.getBoundingClientRect().top : 0;
      });
    },
    open(target: EPUBPreviewTarget, entry = '') {
      this.task.cancel(); this.document = undefined; this.target = target; this.selectedEntry = entry; this.load();
    },
    choose(event: Event) {
      const index = (event.target as HTMLSelectElement).value;
      const target = this.entries[Number(index)]?.target;
      if (index !== '' && target) this.open(target, index);
    },
  },
});
</script>

<template>
  <div class="epub-section-preview">
    <label v-if="navigation" class="epub-contents">
      {{ $t(navigation.source === 'publication' ? 'imports.epub.contents' : 'imports.epub.sections') }}
      <select :value="selectedEntry" @change="choose">
        <option value="" disabled>{{ $t('imports.epub.chooseSection') }}</option>
        <option v-for="(entry, index) in entries" :key="index" :value="String(index)" :disabled="!entry.target">{{ entry.label }}</option>
      </select>
    </label>
    <div v-if="totalSections" class="epub-preview-navigation import-actions">
      <AppButton variant="secondary" :disabled="target.section === 0" @click="open({ section: target.section - 1 })">{{ $t('imports.previous') }}</AppButton>
      <span role="status">{{ $t('imports.epub.sectionPosition', { number: target.section + 1, count: totalSections }) }}</span>
      <AppButton variant="secondary" :disabled="target.section + 1 >= totalSections" @click="open({ section: target.section + 1 })">{{ $t('imports.epub.nextSection') }}</AppButton>
    </div>
    <p class="import-note">{{ $t('imports.epub.previewHint') }}</p>
    <p v-if="task.busy" role="status">{{ $t('imports.epub.loadingPreview') }}</p>
    <div v-if="task.error">
      <p role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
      <AppButton variant="secondary" @click="load">{{ $t('imports.refresh') }}</AppButton>
    </div>
    <div v-if="document" ref="content" class="epub-preview-content" tabindex="0" :aria-label="$t('imports.epub.previewTitle')">
      <ProseRenderer ref="renderer" :document="document" :show-images="true" :fallback-image-alt="$t('imports.epub.illustration')" :image-unavailable="$t('imports.epub.imageUnavailable')" :cover-unavailable="$t('imports.epub.imageUnavailable')" />
    </div>
  </div>
</template>

<style scoped>
.epub-section-preview { min-width: 0; }
.epub-contents { max-width: 40rem; }
.epub-preview-navigation { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); max-width: 40rem; }
.epub-preview-navigation > span { text-align: center; font-size: var(--text-small); }
.epub-preview-content { max-height: 32rem; min-height: 12rem; overflow: auto; overscroll-behavior: contain; scrollbar-gutter: stable; padding: var(--space-4); border-block: 1px solid var(--color-border); font: var(--text-body)/1.8 var(--font-literary); overflow-wrap: anywhere; }
.epub-preview-content :deep(.prose-document) { max-width: 70ch; margin-inline: auto; }
.epub-preview-content :deep(img) { max-height: min(26rem, 60dvh); width: auto; }
.epub-preview-content:focus-visible { outline: 2px solid var(--color-accent); outline-offset: 2px; }
</style>
