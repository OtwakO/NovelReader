<script lang="ts">
import { defineComponent } from 'vue';
import { clearBookSources, deleteBook, getBook, mergeBookSources, type Book } from '../../api/books';
import type { AltSource, Chapter } from '../../api/models';
import { getChapters, switchBookSource } from '../../api/reader';
import AppButton from '../../ui/components/AppButton.vue';
import FeatureScaffold from '../../ui/components/FeatureScaffold.vue';
import SourceRecoveryPanel from '../source-recovery/SourceRecoveryPanel.vue';

export default defineComponent({
  name: 'BookDetailView',
  components: { AppButton, FeatureScaffold, SourceRecoveryPanel },
  data() { return { book: null as Book | null, chapters: [] as Chapter[], loading: true, bookError: '', tocError: '', sourceError: '', sourceMessage: '', switching: false, removing: false, showAllChapters: false, confirmingRemove: false, persistence: Promise.resolve() as Promise<void> }; },
  computed: {
    bookId(): string { return String(this.$route.params.bookId || ''); },
    visibleChapters(): Chapter[] { return this.showAllChapters ? this.chapters : this.chapters.slice(0, 80); },
    progress(): number { if (!this.book?.totalChapterNum) return 0; return Math.min(100, Math.round(((this.book.durChapterIndex + this.book.durChapterPos) / this.book.totalChapterNum) * 100)); },
  },
  watch: {
    bookId() { this.book = null; this.chapters = []; this.showAllChapters = false; void this.load(); },
  },
  async mounted() { await this.load(); },
  methods: {
    async load() {
      this.loading = true; this.bookError = ''; this.tocError = '';
      try { this.book = await getBook(this.bookId); }
      catch (cause) { this.bookError = cause instanceof Error ? cause.message : this.$t('bookDetail.loadFailed'); this.loading = false; return; }
      try { this.chapters = await getChapters(this.bookId); }
      catch (cause) { this.tocError = cause instanceof Error ? cause.message : this.$t('bookDetail.tocFailed'); }
      finally { this.loading = false; }
    },
    persistMatches(sources: AltSource[]) {
      if (!sources.length || !this.book) return;
      this.persistence = this.persistence.then(async () => { if (this.book) this.book = await mergeBookSources(this.book.id, sources); }).catch((cause) => { this.sourceError = cause instanceof Error ? cause.message : this.$t('sourceRecovery.persistFailed'); });
    },
    async clearAndRescan() {
      try { await this.persistence; if (!this.book) throw new Error(this.$t('bookDetail.notFound')); this.book = await clearBookSources(this.book.id); this.sourceMessage = this.$t('sourceRecovery.cleared'); this.sourceError = ''; }
      catch (cause) { this.sourceError = cause instanceof Error ? cause.message : this.$t('sourceRecovery.clearFailed'); throw cause; }
    },
    async selectSource(source: AltSource) {
      if (!this.book || this.switching) return;
      this.switching = true; this.sourceError = ''; this.sourceMessage = '';
      try {
        this.book = await mergeBookSources(this.book.id, [source]);
        const result = await switchBookSource(this.book.id, source.sourceUrl, source.bookUrl);
        this.book = result.book; this.chapters = []; this.tocError = '';
        this.sourceMessage = result.mapping === 'title' ? this.$t('sourceRecovery.switchedTitle') : this.$t('sourceRecovery.switchedIndex');
        try { this.chapters = await getChapters(this.book.id); }
        catch (cause) { this.tocError = cause instanceof Error ? cause.message : this.$t('bookDetail.tocFailed'); }
      } catch (cause) { this.sourceError = cause instanceof Error ? cause.message : this.$t('sourceRecovery.switchFailed'); }
      finally { this.switching = false; }
    },
    async removeBook() {
      if (!this.book || this.removing) return;
      this.removing = true;
      try { await deleteBook(this.book.id); await this.$router.replace('/shelf'); }
      catch (cause) { this.bookError = cause instanceof Error ? cause.message : this.$t('bookDetail.removeFailed'); }
      finally { this.removing = false; this.confirmingRemove = false; }
    },
  },
});
</script>

<template>
  <FeatureScaffold :title="book?.name || $t('bookDetail.title')" :description="$t('bookDetail.description')">
    <p v-if="loading" aria-busy="true">{{ $t('bookDetail.loading') }}</p>
    <section v-else-if="!book" class="state"><p role="alert">{{ bookError || $t('bookDetail.notFound') }}</p><RouterLink to="/shelf">{{ $t('bookDetail.back') }}</RouterLink></section>
    <template v-else>
      <p v-if="bookError" class="banner-error" role="alert">{{ bookError }}</p>
      <section class="hero">
        <img v-if="book.coverUrl" :src="book.coverUrl" :alt="$t('bookDetail.coverAlt', { name: book.name })" class="cover">
        <div v-else class="cover placeholder" aria-hidden="true">{{ book.name.slice(0, 1) }}</div>
        <div class="identity"><p class="eyebrow">{{ $t('bookDetail.onShelf') }}</p><h2>{{ book.name }}</h2><p class="author">{{ book.author || $t('app.common.unknownAuthor') }}</p><div class="metadata"><span v-if="book.kind">{{ book.kind }}</span><span v-if="book.wordCount">{{ book.wordCount }}</span><span>{{ $t('bookDetail.tocEntries', { count: chapters.length }) }}</span><span v-if="book.totalChapterNum">{{ $t('bookDetail.progress', { percent: progress }) }}</span></div><p v-if="book.lastChapter" class="latest">{{ $t('bookDetail.latest', { chapter: book.lastChapter }) }}</p><p class="source">{{ $t('bookDetail.currentSource', { source: book.origin || book.sourceUrl }) }}</p><div class="actions"><RouterLink class="primary-link" :to="`/books/${encodeURIComponent(book.id)}/read/${book.durChapterIndex}`">{{ $t('bookDetail.continue') }}</RouterLink><AppButton variant="danger" @click="confirmingRemove = true">{{ $t('bookDetail.remove') }}</AppButton></div></div>
      </section>
      <section v-if="confirmingRemove" class="confirmation" role="alertdialog" :aria-label="$t('bookDetail.confirmRemoveTitle')"><strong>{{ $t('bookDetail.confirmRemoveTitle') }}</strong><p>{{ $t('bookDetail.confirmRemoveDescription', { name: book.name }) }}</p><div><AppButton variant="secondary" @click="confirmingRemove = false">{{ $t('bookDetail.cancel') }}</AppButton><AppButton variant="danger" :busy="removing" @click="removeBook">{{ $t('bookDetail.confirmRemove') }}</AppButton></div></section>
      <section v-if="book.intro" class="panel"><h2>{{ $t('bookDetail.synopsis') }}</h2><p class="intro">{{ book.intro }}</p></section>
      <section class="panel toc-panel"><header class="toc-header"><h2>{{ $t('bookDetail.chapters') }}</h2><span>{{ $t('bookDetail.tocEntries', { count: chapters.length }) }}</span></header><p v-if="tocError" class="banner-error" role="alert">{{ tocError }}</p><ol v-if="chapters.length" class="chapter-list"><li v-for="chapter in visibleChapters" :key="chapter.index" :class="{ volume: chapter.isVolume }"><span v-if="chapter.isVolume">{{ chapter.title }}</span><RouterLink v-else :to="`/books/${encodeURIComponent(book.id)}/read/${chapter.index}`">{{ chapter.title }}</RouterLink></li></ol><p v-else-if="!tocError" class="muted">{{ $t('bookDetail.noChapters') }}</p><AppButton v-if="chapters.length > visibleChapters.length" variant="quiet" @click="showAllChapters = true">{{ $t('bookDetail.showAll', { count: chapters.length }) }}</AppButton></section>
      <SourceRecoveryPanel :book="{ name: book.name, author: book.author }" :current-source-url="book.sourceUrl" :stored-sources="book.alternateSources || []" :switching="switching" :action-error="sourceError" :action-message="sourceMessage" :on-clear-and-rescan="clearAndRescan" @matches="persistMatches" @select="selectSource" />
    </template>
  </FeatureScaffold>
</template>

<style scoped>
.state, .panel, .confirmation { padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.hero { display: grid; grid-template-columns: 9rem minmax(0,1fr); gap: 1.5rem; align-items: start; padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.cover { width: 9rem; aspect-ratio: 2/3; object-fit: cover; border-radius: var(--radius-md); }
.placeholder { display: grid; place-items: center; background: linear-gradient(145deg,var(--color-accent),var(--color-accent-strong)); color: white; font: 700 2.5rem var(--font-literary); }
.identity { min-width: 0; }
.eyebrow { margin: 0; color: var(--color-warm); font-size: .72rem; font-weight: 800; letter-spacing: .1em; text-transform: uppercase; }
.identity h2 { margin: .25rem 0; font: 700 clamp(1.7rem,4vw,2.7rem)/1.12 var(--font-literary); }
.author,.latest,.source,.muted { color: var(--color-ink-muted); overflow-wrap: anywhere; }
.metadata,.actions { display: flex; flex-wrap: wrap; gap: .5rem; margin-top: .8rem; }
.metadata span { padding: .3rem .55rem; border-radius: 999px; background: var(--color-paper-muted); font-size: .76rem; }
.primary-link { min-height: 2.75rem; display: inline-flex; align-items: center; border-radius: var(--radius-md); padding: .65rem 1rem; background: var(--color-accent); color: white; text-decoration: none; font-weight: 700; }
.panel { margin-top: 1rem; }
.panel header { display: flex; justify-content: space-between; gap: 1rem; align-items: center; }
.panel h2 { margin: 0; font: 700 1.15rem var(--font-literary); }
.panel header span { color: var(--color-ink-muted); font-size: .82rem; }
.intro { line-height: 1.75; white-space: pre-line; }
.toc-panel { padding: 0; overflow: hidden; }
.toc-header { min-height: 3.5rem; padding: .9rem 1rem; border-bottom: 1px solid var(--color-border); }
.toc-header span { flex: 0 0 auto; white-space: nowrap; }
.toc-panel > .banner-error, .toc-panel > .muted { margin: 1rem; }
.chapter-list { counter-reset: readable-chapter; display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); margin: 0; padding: 0; list-style: none; }
.chapter-list li { min-width: 0; display: grid; grid-template-columns: 2.5rem minmax(0,1fr); align-items: start; gap: .65rem; min-height: 3rem; padding: .75rem 1rem; border-bottom: 1px solid var(--color-border); }
.chapter-list li:not(.volume) { counter-increment: readable-chapter; }
.chapter-list li:not(.volume)::before { content: counter(readable-chapter,decimal-leading-zero); padding-top: .08rem; color: var(--color-ink-muted); font-size: .72rem; font-variant-numeric: tabular-nums; line-height: 1.55; text-align: right; }
.chapter-list li:nth-last-child(-n+2):not(.volume) { border-bottom: 0; }
.chapter-list a { min-width: 0; color: var(--color-accent-strong); line-height: 1.55; overflow-wrap: anywhere; text-decoration: none; }
.chapter-list a:hover { text-decoration: underline; text-underline-offset: .18em; }
.chapter-list .volume { grid-column: 1/-1; grid-template-columns: minmax(0,1fr); min-height: auto; padding-block: .65rem; background: var(--color-paper-muted); color: var(--color-ink-muted); font-size: .78rem; font-weight: 800; line-height: 1.45; overflow-wrap: anywhere; }
.toc-panel > :deep(.app-button) { margin: .75rem 1rem 1rem; }
.confirmation { margin-top: 1rem; }
.confirmation p { color: var(--color-ink-muted); }
.confirmation div { display: flex; gap: .5rem; }
.banner-error { padding: .7rem; border-radius: var(--radius-md); background: #f8e4df; color: var(--color-danger); }
:deep(.recovery) { margin-top: 1rem; }
@media (max-width: 38rem) {
  .hero { grid-template-columns: 6rem minmax(0,1fr); gap: 1rem; }
  .cover { width: 6rem; }
  .actions { grid-column: 1/-1; }
  .toc-header { align-items: baseline; }
  .chapter-list { grid-template-columns: minmax(0,1fr); }
  .chapter-list li { grid-template-columns: 2.25rem minmax(0,1fr); padding-inline: .85rem; }
  .chapter-list li:nth-last-child(-n+2):not(.volume) { border-bottom: 1px solid var(--color-border); }
  .chapter-list li:last-child:not(.volume) { border-bottom: 0; }
}
</style>
