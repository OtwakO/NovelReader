<script lang="ts">
import { defineComponent } from 'vue';
import { previewBook, shelveBook, type BookPreview } from '../../api/books';
import type { SearchResult } from '../../api/search';
import AppButton from '../../ui/components/AppButton.vue';
import FeatureScaffold from '../../ui/components/FeatureScaffold.vue';
import { loadPreviewSelection } from '../search/search-preview';

export default defineComponent({
  name: 'BookPreviewView', components: { AppButton, FeatureScaffold },
  data() { return { candidate: null as SearchResult | null, preview: null as BookPreview | null, loading: true, shelving: false, error: '', shelfError: '', addedBookId: '' }; },
  async mounted() { await this.load(); },
  methods: {
    async load() {
      const key = typeof this.$route.query.preview === 'string' ? this.$route.query.preview : '';
      this.candidate = loadPreviewSelection(key);
      if (!this.candidate) { this.loading = false; this.error = this.$t('bookPreview.missing'); return; }
      this.loading = true; this.error = '';
      try { this.preview = await previewBook(this.candidate); }
      catch (cause) { this.error = cause instanceof Error ? cause.message : this.$t('bookPreview.failed'); }
      finally { this.loading = false; }
    },
    async addToShelf() {
      if (!this.candidate || this.addedBookId) return;
      this.shelving = true; this.shelfError = '';
      try {
        const id = crypto.randomUUID?.() ?? `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`;
        const book = await shelveBook({
          ...this.candidate,
          ...this.preview?.book,
          id,
          sourceName: this.candidate.sourceName,
          alternateSources: this.candidate.alternateSources,
        });
        this.addedBookId = book.id;
      } catch (cause) { this.shelfError = cause instanceof Error ? cause.message : this.$t('bookPreview.shelfFailed'); }
      finally { this.shelving = false; }
    },
  },
});
</script>

<template>
  <FeatureScaffold :title="candidate?.name || $t('bookPreview.title')" :description="$t('bookPreview.description')">
    <p v-if="loading" aria-busy="true">{{ $t('bookPreview.loading') }}</p>
    <section v-else-if="error" class="state"><p role="alert">{{ error }}</p><RouterLink to="/search">{{ $t('bookPreview.back') }}</RouterLink></section>
    <template v-else-if="candidate && preview">
      <section class="hero">
        <img v-if="preview.book.coverUrl" :src="preview.book.coverUrl" alt="" class="cover">
        <div v-else class="cover placeholder" aria-hidden="true">{{ preview.book.name.slice(0, 1) }}</div>
        <div class="copy"><p class="author">{{ preview.book.author || $t('app.common.unknownAuthor') }}</p><p class="meta">{{ preview.book.kind }}<span v-if="preview.book.lastChapter"> · {{ preview.book.lastChapter }}</span></p><p class="source">{{ $t('bookPreview.source', { source: candidate.sourceName }) }} · {{ $t('bookPreview.sourceCount', { count: 1 + (candidate.alternateSources?.length ?? 0) }) }}</p><p class="intro">{{ preview.book.intro || $t('bookPreview.noIntro') }}</p></div>
      </section>
      <section class="toc"><h2>{{ $t('bookPreview.chapters', { count: preview.chapters.length }) }}</h2><ol><li v-for="chapter in preview.chapters.slice(0, 12)" :key="chapter.index">{{ chapter.title }}</li></ol><p v-if="preview.chapters.length > 12">{{ $t('bookPreview.moreChapters', { count: preview.chapters.length - 12 }) }}</p></section>
      <p v-if="shelfError" class="error" role="alert">{{ shelfError }}</p>
      <div class="actions"><AppButton v-if="!addedBookId" :busy="shelving" @click="addToShelf">{{ shelving ? $t('bookPreview.shelving') : $t('bookPreview.shelve') }}</AppButton><RouterLink v-else class="primary-link" :to="`/books/${encodeURIComponent(addedBookId)}`">{{ $t('bookPreview.openShelfBook') }}</RouterLink><RouterLink to="/search">{{ $t('bookPreview.back') }}</RouterLink></div>
    </template>
  </FeatureScaffold>
</template>

<style scoped>
.state, .toc { padding: 1.25rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }.hero { display: grid; grid-template-columns: 9rem minmax(0, 1fr); gap: 1.5rem; align-items: start; margin-bottom: 1rem; }.cover { width: 9rem; aspect-ratio: 2 / 3; object-fit: cover; border-radius: var(--radius-md); box-shadow: var(--shadow-card); }.placeholder { display: grid; place-items: center; background: linear-gradient(145deg, var(--color-accent), var(--color-accent-strong)); color: white; font: 700 2.5rem var(--font-literary); }.copy { min-width: 0; }.author, .meta, .source { color: var(--color-ink-muted); }.source { color: var(--color-accent); }.intro { max-width: 55rem; line-height: 1.75; white-space: pre-line; }.toc h2 { font-family: var(--font-literary); }.toc ol { columns: 2; padding-left: 1.5rem; }.toc li { break-inside: avoid; margin-bottom: .5rem; }.actions { display: flex; flex-wrap: wrap; align-items: center; gap: 1rem; margin-top: 1rem; }.primary-link { min-height: 2.75rem; display: inline-flex; align-items: center; border-radius: var(--radius-md); padding: .65rem 1rem; background: var(--color-accent); color: white; text-decoration: none; font-weight: 700; }.error { color: var(--color-danger); }
@media (max-width: 38rem) { .hero { grid-template-columns: 6rem minmax(0, 1fr); gap: 1rem; }.cover { width: 6rem; }.intro { grid-column: 1 / -1; }.toc ol { columns: 1; } }
</style>
