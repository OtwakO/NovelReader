<script lang="ts">
import { defineComponent, nextTick, type PropType } from 'vue';
import type { Chapter } from '../../api/models';
import AppButton from '../../ui/components/AppButton.vue';
import TocChapterList from './TocChapterList.vue';
import { readableChapterCount, visibleTocChapters, type TocOrder } from './reader-toc';

export default defineComponent({
  name: 'ReaderTocSheet',
  components: { AppButton, TocChapterList },
  props: {
    chapters: { type: Array as PropType<Chapter[]>, default: () => [] },
    currentIndex: { type: Number, required: true },
  },
  emits: ['open', 'close'],
  data() { return { query: '', order: 'ascending' as TocOrder }; },
  computed: {
    readableCount(): number { return readableChapterCount(this.chapters); },
    visibleChapters(): Chapter[] { return visibleTocChapters(this.chapters, this.query, this.order); },
    currentVisible(): boolean { return this.visibleChapters.some(chapter => chapter.index === this.currentIndex); },
    summaryKey(): string { return this.readableCount === this.chapters.length ? 'reader.toc.readableSummary' : 'reader.toc.summary'; },
  },
  mounted() { void this.scrollToCurrent(false); },
  methods: {
    toggleOrder() {
      this.order = this.order === 'ascending' ? 'descending' : 'ascending';
      void this.scrollToCurrent(false);
    },
    clearSearch() {
      this.query = '';
      void this.scrollToCurrent(false);
    },
    async scrollToCurrent(smooth = true) {
      if (!this.currentVisible) this.query = '';
      await nextTick();
      const list = this.$refs.list as HTMLElement | undefined;
      const row = list?.querySelector<HTMLElement>('[data-current="true"]');
      if (!list || !row) return;
      const top = Math.max(0, row.offsetTop - (list.clientHeight - row.offsetHeight) / 2);
      if (smooth && typeof list.scrollTo === 'function') list.scrollTo({ top, behavior: 'smooth' });
      else list.scrollTop = top;
      row.querySelector<HTMLElement>('button, a')?.focus({ preventScroll: true });
    },
  },
});
</script>

<template>
  <div class="overlay" @click.self="$emit('close')" @keydown.esc="$emit('close')">
    <section class="sheet" role="dialog" aria-modal="true" :aria-label="$t('reader.toc.title')">
      <div class="sheet-toolbar">
<header>
        <div>
          <h2>{{ $t('reader.toc.title') }}</h2>
          <p>{{ $t(summaryKey, { readable: readableCount, total: chapters.length }) }}</p>
        </div>
        <AppButton variant="quiet" @click="$emit('close')">{{ $t('reader.close') }}</AppButton>
      </header>
      <div class="toc-tools">
        <label class="search-field">
          <span>{{ $t('reader.toc.search') }}</span>
          <span class="search-input">
            <input v-model="query" type="search" :placeholder="$t('reader.toc.searchPlaceholder')">
            <button v-if="query" type="button" :aria-label="$t('reader.toc.clearSearch')" @click="clearSearch">×</button>
          </span>
        </label>
        <div class="tool-actions">
          <AppButton variant="secondary" @click="toggleOrder">{{ order === 'ascending' ? $t('reader.toc.descending') : $t('reader.toc.ascending') }}</AppButton>
          <AppButton variant="secondary" @click="scrollToCurrent(true)">{{ $t('reader.toc.jumpCurrent') }}</AppButton>
        </div>
      </div>
      <p v-if="query" class="result-count" role="status">{{ $t('reader.toc.matches', { count: visibleChapters.length }) }}</p>
</div>
      <div ref="list" class="toc-list">
        <TocChapterList v-if="visibleChapters.length" :chapters="visibleChapters" :current-index="currentIndex" @open="$emit('open', $event)" />
      <section v-else class="no-matches">
        <p>{{ $t('reader.toc.noMatches') }}</p>
        <AppButton variant="secondary" @click="clearSearch">{{ $t('reader.toc.clearSearch') }}</AppButton>
      </section>
</div>
    </section>
  </div>
</template>

<style scoped>
.overlay{position:fixed;z-index:90;inset:0;display:flex;align-items:flex-end;justify-content:center;background:rgb(0 0 0/.38)}
.sheet{width:min(48rem,100%);height:min(86dvh,48rem);display:grid;grid-template-rows:auto minmax(0,1fr);overflow:hidden;border-radius:var(--radius-lg) var(--radius-lg) 0 0;background:var(--color-paper-raised);color:var(--color-ink);box-shadow:0 -20px 60px rgb(31 29 25/.2)}.sheet-toolbar{position:relative;z-index:2;padding:1rem 1rem .7rem;border-bottom:1px solid var(--color-border);background:var(--color-paper-raised);box-shadow:0 .4rem 1rem rgb(31 29 25/.05)}header{display:flex;justify-content:space-between;align-items:center;gap:1rem;padding:0 0 .75rem}h2,p{margin:0}h2{font:700 1.35rem var(--font-literary)}header p,.result-count{color:var(--color-ink-muted)}
.toc-tools{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:.75rem;align-items:end}.toc-list{min-height:0;overflow:auto;overscroll-behavior:contain;scrollbar-color:var(--color-border) transparent;background:var(--color-paper-raised)}.search-field{display:grid;gap:.3rem}.search-field>span:first-child{color:var(--color-ink-muted);font-size:.75rem;font-weight:700}.search-input{position:relative}.search-input input{width:100%;min-height:2.75rem;border:1px solid var(--color-border);border-radius:var(--radius-md);padding:.6rem 2.75rem .6rem .75rem;background:white;color:var(--color-ink)}.search-input button{position:absolute;right:.25rem;top:.25rem;width:2.25rem;height:2.25rem;border:0;border-radius:var(--radius-sm);background:transparent;color:var(--color-ink-muted);font-size:1.25rem}.tool-actions{display:flex;gap:.5rem}.result-count{padding:.15rem 0 .65rem;font-size:.8rem;font-variant-numeric:tabular-nums}
.no-matches{display:grid;justify-items:center;gap:.75rem;padding:2rem 1rem;text-align:center}.no-matches p{color:var(--color-ink-muted)}
@media(max-width:38rem){.sheet{height:92dvh}.toc-tools{grid-template-columns:1fr}.tool-actions{display:grid;grid-template-columns:1fr 1fr}.tool-actions :deep(button){width:100%}.sheet{max-height:92dvh}.overlay{padding:0}}
</style>
