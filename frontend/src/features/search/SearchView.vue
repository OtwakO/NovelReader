<script lang="ts">
import { defineComponent } from 'vue';
import type { SearchResult } from '../../api/search';
import FeatureScaffold from '../../ui/components/FeatureScaffold.vue';
import AppButton from '../../ui/components/AppButton.vue';
import SearchControls from './components/SearchControls.vue';
import SearchResultCard from './components/SearchResultCard.vue';
import SearchStatus from './components/SearchStatus.vue';
import { createCandidateSelectionKey, saveCandidateSelection } from './candidate-selection';
import { useSearchStore } from './search-store';

export default defineComponent({
  name: 'SearchView',
  components: { FeatureScaffold, AppButton, SearchControls, SearchResultCard, SearchStatus },
  data() { return { search: useSearchStore() }; },
  mounted() { this.search.initialize(); },
  beforeUnmount() { this.search.stop(); },
  methods: {
    key(result: SearchResult) { return `${result.sourceUrl}\u0000${result.bookUrl}`; },
    submit() { this.search.search(); },
    open(result: SearchResult) {
      const selectionKey = createCandidateSelectionKey();
      saveCandidateSelection(selectionKey, result);
      this.search.save();
      void this.$router.push({ name: 'candidate-book-detail', query: { candidate: selectionKey } });
    },
  },
});
</script>

<template>
  <FeatureScaffold :title="$t('search.title')" :description="$t('search.description')">
    <div class="search-layout">
      <form class="search-form" role="search" @submit.prevent="submit">
        <label class="sr-only" for="book-search">{{ $t('search.form.label') }}</label>
        <input id="book-search" v-model="search.query" type="search" :placeholder="$t('search.form.placeholder')" autocomplete="off">
        <AppButton v-if="search.searching" variant="danger" @click="search.stop()">{{ $t('search.actions.stop') }}</AppButton>
        <AppButton v-else type="submit" :disabled="!search.query.trim()">{{ $t('search.actions.search') }}</AppButton>
      </form>
      <SearchControls v-model:batch-size="search.batchSize" v-model:intensity="search.intensity" v-model:advanced-concurrency="search.advancedConcurrency" @change="search.persistPreferences()" />
      <SearchStatus v-if="search.searchedQuery" :checked="search.checked" :eligible="search.eligible" :result-count="search.resultCount" :searching="search.searching" :concurrency="search.effectiveConcurrency || search.activeConcurrency" :source-failures="search.sourceFailures" :error-code="search.errorCode" :error-detail="search.errorDetail" :storage-warning="search.storageWarning" :restart-required="search.restartRequired" :retry-required="search.retryRequired" :has-more="search.hasMore" :more-count="search.moreCount" @restart="search.restart()" @retry="search.retry()" @more="search.more()" />

      <section v-if="search.results.length" class="results" :aria-label="$t('search.results.label')">
        <header><strong>{{ $t('search.results.summary', { count: search.resultCount }) }}</strong><span v-if="search.multipleSourceCount">{{ $t('search.results.multiple', { count: search.multipleSourceCount }) }}</span></header>
        <div class="result-list">
          <div v-for="result in search.results" :key="key(result)">
            <SearchResultCard
              :result="result"
              :can-continue-search="search.hasMore"
              :continue-search-count="search.moreCount"
              :search-scanning="search.searching"
              :search-retry-required="search.retryRequired"
              :search-restart-required="search.restartRequired"
              @open="open(result)"
              @continue-search="search.more()"
              @retry-search="search.retry()"
              @restart-search="search.restart()"
            />
          </div>
        </div>
      </section>
      <section v-else-if="search.searchedQuery && !search.searching && !search.retryRequired && !search.hasMore" class="empty"><h2>{{ $t('search.empty.title') }}</h2><p>{{ $t('search.empty.description') }}</p></section>
    </div>
  </FeatureScaffold>
</template>

<style scoped>
.search-layout { display: grid; gap: 1rem; }.search-form { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: .65rem; }.search-form input { min-width: 0; min-height: 3rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .7rem .9rem; background: var(--color-paper-raised); color: var(--color-ink); font-size: 1rem; }
.results header { display: flex; flex-wrap: wrap; justify-content: space-between; gap: .5rem; margin-bottom: .75rem; color: var(--color-ink-muted); }.result-list { display: grid; gap: .7rem; }.notice { margin: .35rem .75rem 0; font-size: .82rem; }.success { color: var(--color-success); }.error { color: var(--color-danger); }.empty { padding: 2rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); text-align: center; }.empty h2 { font-family: var(--font-literary); }.empty p { color: var(--color-ink-muted); }
@media (max-width: 30rem) { .search-form { grid-template-columns: 1fr; }.search-form :deep(.app-button) { width: 100%; } }
</style>
