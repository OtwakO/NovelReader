<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import AppButton from '../../ui/components/AppButton.vue';
import AppIcon from '../../ui/components/AppIcon.vue';

interface ContentsEntry { key: string; label: string; disabled?: boolean; depth?: number }

// Presentation only: formats retain loading, generation and selection ownership.
export default defineComponent({
  components: { AppButton, AppIcon },
  props: {
    entries: { type: Array as PropType<ContentsEntry[]>, required: true },
    selected: { type: String, default: '' },
    section: { type: Number, required: true },
    totalSections: { type: Number, required: true },
    contentsLabel: { type: String, required: true },
    disabled: Boolean,
  },
  emits: ['select', 'previous', 'next'],
  watch: {
    selected: { flush: 'post', handler() { this.revealSelection(); } },
    entries: { flush: 'post', handler() { this.revealSelection(); } },
  },
  methods: {
    revealSelection() {
      const list = this.$refs.contents as HTMLElement;
      const active = list?.querySelector<HTMLElement>('[aria-current="true"]');
      if (!active) return;
      const item = active.getBoundingClientRect(), bounds = list.getBoundingClientRect();
      if (item.bottom > bounds.bottom) list.scrollTop += item.bottom - bounds.bottom;
      else if (item.top < bounds.top) list.scrollTop += item.top - bounds.top;
    },
    resetScroll(anchor?: HTMLElement) {
      const panel = this.$refs.content as HTMLElement;
      panel.scrollTop = anchor ? panel.scrollTop + anchor.getBoundingClientRect().top - panel.getBoundingClientRect().top : 0;
    },
  },
});
</script>

<template>
  <div class="book-preview">
    <div class="preview-frame">
      <nav class="preview-contents" :aria-label="contentsLabel">
        <h4>{{ contentsLabel }}</h4>
        <ol ref="contents" class="preview-entries">
          <li v-for="entry in entries" :key="entry.key">
            <button type="button" :disabled="disabled || entry.disabled" :aria-current="selected === entry.key ? 'true' : undefined" :style="{ paddingInlineStart: `${.75 + Math.min(entry.depth || 0, 6) * .75}rem` }" @click="$emit('select', entry.key)">{{ entry.label }}</button>
          </li>
        </ol>
        <slot name="contents-footer" />
      </nav>
      <div class="preview-main">
        <div class="preview-navigation">
          <AppButton variant="secondary" :disabled="disabled || section === 0 || !totalSections" :aria-label="$t('imports.sectionPreview.previous')" :title="$t('imports.sectionPreview.previous')" @click="$emit('previous')"><AppIcon name="previous" /></AppButton>
          <span role="status">{{ $t('imports.sectionPreview.position', { number: totalSections ? section + 1 : 0, count: totalSections }) }}</span>
          <AppButton variant="secondary" :disabled="disabled || section + 1 >= totalSections" :aria-label="$t('imports.sectionPreview.next')" :title="$t('imports.sectionPreview.next')" @click="$emit('next')"><AppIcon name="next" /></AppButton>
        </div>
        <div ref="content" class="preview-content" tabindex="0" :aria-label="$t('imports.sectionPreview.title')"><slot /></div>
      </div>
    </div>
    <slot name="selection-action" />
  </div>
</template>

<style scoped>
.book-preview { container-type: inline-size; min-width: 0; }
.preview-frame { display: grid; grid-template-columns: 250px minmax(0, 1fr); border: 1px solid var(--color-border); }
.preview-contents { min-width: 0; padding: var(--space-3); background: var(--color-paper); border-inline-end: 1px solid var(--color-border); }
.preview-contents h4 { margin: 0 0 var(--space-3); font-size: var(--text-small); }
.preview-entries { max-height: 30rem; overflow: auto; margin: 0; padding: 0; list-style: none; overscroll-behavior: contain; }
.preview-entries button { width: 100%; min-height: 44px; padding: .75rem; border: 0; border-radius: var(--radius-sm); background: transparent; color: var(--color-ink); font: inherit; font-size: var(--text-small); text-align: start; overflow-wrap: anywhere; cursor: pointer; }
.preview-entries button:hover:not(:disabled) { background: var(--color-paper-muted); }
.preview-entries button[aria-current='true'] { background: var(--color-accent-soft); color: var(--color-accent); font-weight: var(--weight-strong); }
.preview-entries button:disabled { opacity: .55; cursor: default; }
.preview-main { min-width: 0; }
.preview-navigation { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); padding: var(--space-3); border-bottom: 1px solid var(--color-border); }
.preview-navigation > span { text-align: center; font-size: var(--text-small); font-variant-numeric: tabular-nums; }
.preview-navigation .app-button { min-width: 44px; min-height: 44px; padding: .625rem; }
.preview-content { height: 30rem; overflow: auto; overscroll-behavior: contain; scrollbar-gutter: stable both-edges; padding: var(--space-5); background: var(--color-paper-raised); overflow-wrap: anywhere; }
.preview-content :deep(.prose-document), .preview-content :deep(.preview-text) { max-width: 70ch; margin-inline: auto; font: var(--text-body)/1.8 var(--font-literary); }
.preview-content :deep(.preview-text) { white-space: pre-wrap; margin-block: 0; }
.preview-content :deep(img) { max-height: min(26rem, 60dvh); width: auto; }
.preview-content:focus-visible, .preview-entries button:focus-visible { outline: 2px solid var(--color-accent); outline-offset: -2px; }
@container (max-width: 620px) {
  .preview-frame { grid-template-columns: minmax(0, 1fr); }
  .preview-contents { border-inline-end: 0; border-bottom: 1px solid var(--color-border); }
  .preview-entries { max-height: 10rem; }
  .preview-content { height: 26rem; padding: var(--space-4); }
}
</style>
