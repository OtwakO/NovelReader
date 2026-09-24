<script lang="ts">
import { defineComponent } from 'vue';
import AppButton from '../../ui/components/AppButton.vue';
import ProseRenderer from '../reader/ProseRenderer.vue';
import BookPreview from './BookPreview.vue';
import { getEPUBPreviewNavigation, getEPUBPreviewSection, type EPUBPreviewEntry, type EPUBPreviewNavigation, type EPUBPreviewTarget } from '../../api/epub-review';
import type { StructuredProseDocument } from '../../api/structured-prose';
import { createImportTask } from './import-task';
import { importErrorKey } from './import-feedback';

export default defineComponent({
  components: { AppButton, ProseRenderer, BookPreview },
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
    activeEntry(): string { return this.selectedEntry || this.entries.find(entry => entry.target?.section === this.target.section && (entry.target.anchor || '') === (this.target.anchor || ''))?.key || ''; },
    entries(): { key: string; label: string; depth: number; disabled: boolean; target?: EPUBPreviewTarget }[] {
      const result: { key: string; label: string; depth: number; disabled: boolean; target?: EPUBPreviewTarget }[] = [];
      const visit = (entries: EPUBPreviewEntry[], depth: number) => {
        for (const entry of entries) {
          const label = entry.label.trim() || (entry.target ? this.$t('imports.epub.sectionNumber', { number: entry.target.section + 1 }) : this.$t('imports.epub.untitledEntry'));
          result.push({ key: String(result.length), label, depth, disabled: entry.unavailable || !entry.target, target: entry.unavailable ? undefined : entry.target });
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
        const panel = this.$refs.preview as InstanceType<typeof BookPreview>;
        const renderer = this.$refs.renderer as InstanceType<typeof ProseRenderer>;
        const anchor = this.target.anchor ? renderer.findAnchor(this.target.anchor) : undefined;
        panel.resetScroll(anchor);
      });
    },
    open(target: EPUBPreviewTarget, entry = '') {
      this.task.cancel(); this.document = undefined; this.target = target; this.selectedEntry = entry; this.load();
    },
    choose(index: string) {
      const target = this.entries[Number(index)]?.target;
      if (index !== '' && target) this.open(target, index);
    },
  },
});
</script>

<template>
  <div class="epub-section-preview">
    <BookPreview ref="preview" :entries="entries" :selected="activeEntry" :section="target.section" :total-sections="totalSections" :contents-label="$t(navigation?.source === 'sections' ? 'imports.epub.sections' : 'imports.sectionPreview.contents')" @select="choose" @previous="open({ section: target.section - 1 })" @next="open({ section: target.section + 1 })">
      <p v-if="task.busy" role="status">{{ $t('imports.sectionPreview.loading') }}</p>
      <div v-if="task.error">
        <p role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
        <AppButton variant="secondary" @click="load">{{ $t('imports.refresh') }}</AppButton>
      </div>
      <ProseRenderer v-if="document" ref="renderer" :document="document" :show-images="true" :fallback-image-alt="$t('imports.epub.illustration')" :image-unavailable="$t('imports.epub.imageUnavailable')" :cover-unavailable="$t('imports.epub.imageUnavailable')" />
    </BookPreview>
    <p class="import-note">{{ $t('imports.epub.previewHint') }}</p>
  </div>
</template>
