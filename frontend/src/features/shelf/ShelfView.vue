<script lang="ts">
import { defineComponent } from 'vue';
import { listBooks, type Book } from '../../api/client';
import FeatureScaffold from '../../ui/components/FeatureScaffold.vue';
import AppButton from '../../ui/components/AppButton.vue';

export default defineComponent({
  name: 'ShelfView',
  components: { FeatureScaffold, AppButton },
  data() { return { books: [] as Book[], loading: true, error: '' }; },
  async mounted() { await this.load(); },
  methods: {
    async load() {
      this.loading = true; this.error = '';
      try { this.books = await listBooks(); }
      catch (cause) { this.error = cause instanceof Error ? cause.message : this.$t('shelf.failed'); }
      finally { this.loading = false; }
    },
  },
});
</script>

<template>
  <FeatureScaffold :title="$t('shelf.title')" :description="$t('shelf.description')">
    <p v-if="loading" aria-busy="true">{{ $t('shelf.loading') }}</p>
    <section v-else-if="error" class="state"><p role="alert">{{ error }}</p><AppButton variant="secondary" @click="load">{{ $t('app.common.retry') }}</AppButton></section>
    <section v-else-if="books.length === 0" class="state"><h2>{{ $t('shelf.emptyTitle') }}</h2><p>{{ $t('shelf.emptyDescription') }}</p><div><RouterLink to="/explore">{{ $t('shelf.explore') }}</RouterLink><RouterLink to="/search">{{ $t('shelf.search') }}</RouterLink></div></section>
    <section v-else class="book-grid" :aria-label="$t('shelf.booksLabel')">
      <RouterLink v-for="book in books" :key="book.id" class="book-card" :to="`/books/${encodeURIComponent(book.id)}`">
        <div class="cover" aria-hidden="true">{{ book.name.slice(0, 1) }}</div>
        <div><h2>{{ book.name }}</h2><p>{{ book.author || $t('app.common.unknownAuthor') }}</p><small>{{ $t('shelf.chapter', { chapter: Math.max(1, book.durChapterIndex + 1) }) }}</small></div>
      </RouterLink>
    </section>
  </FeatureScaffold>
</template>

<style scoped>
.state { padding: 2rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); text-align: center; }
.state div { display: flex; justify-content: center; gap: 1rem; }
.book-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(14rem, 1fr)); gap: 1rem; }
.book-card { min-width: 0; display: grid; grid-template-columns: 4.5rem minmax(0, 1fr); gap: 1rem; padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); color: var(--color-ink); text-decoration: none; }
.cover { aspect-ratio: 2 / 3; display: grid; place-items: center; border-radius: var(--radius-sm); background: linear-gradient(145deg, var(--color-accent), var(--color-accent-strong)); color: white; font: 700 1.6rem var(--font-literary); }
h2 { margin: .15rem 0 .3rem; overflow: hidden; text-overflow: ellipsis; font: 700 1.05rem var(--font-literary); white-space: nowrap; }
.book-card p, small { color: var(--color-ink-muted); }
.book-card p { margin: 0 0 .75rem; }
</style>
