<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { SearchResult } from '../../../api/search';
import AppButton from '../../../ui/components/AppButton.vue';
import BookCover from '../../books/BookCover.vue';
import CandidateShelfAction from '../../candidates/CandidateShelfAction.vue';

export default defineComponent({
  name: 'SearchResultCard',
  components: { AppButton, BookCover, CandidateShelfAction },
  props: {
    result: { type: Object as PropType<SearchResult>, required: true },
    canContinueSearch: { type: Boolean, default: false },
    continueSearchCount: { type: Number, default: 0 },
    searchScanning: { type: Boolean, default: false },
    searchRetryRequired: { type: Boolean, default: false },
    searchRestartRequired: { type: Boolean, default: false },
  },
  emits: ['open', 'open-shelf', 'continue-search', 'retry-search', 'restart-search'],
  computed: { sourceCount(): number { return 1 + (this.result.alternateSources?.length ?? 0); } },
});
</script>

<template>
  <article class="result-card">
    <button type="button" class="main" :aria-label="$t('search.results.detailsFor', { name: result.name })" @click="$emit(result.shelfBookId ? 'open-shelf' : 'open')">
      <BookCover class="cover" :name="result.name" :url="result.coverDisplayUrl || ''" alt="" />
      <span class="info"><strong>{{ result.name }}</strong><span>{{ result.author || $t('app.common.unknownAuthor') }}</span><span v-if="result.lastChapter" class="chapter">{{ result.lastChapter }}</span><span class="source">{{ result.sourceName }} · {{ $t('search.results.sources', { count: sourceCount }) }}</span></span>
    </button>
    <AppButton class="preview-action" variant="secondary" @click="$emit(result.shelfBookId ? 'open-shelf' : 'open')">{{ $t('search.results.preview') }}</AppButton>
    <CandidateShelfAction
      :result="result"
      :can-continue-search="canContinueSearch"
      :continue-search-count="continueSearchCount"
      :search-scanning="searchScanning"
      :search-retry-required="searchRetryRequired"
      :search-restart-required="searchRestartRequired"
      @continue-search="$emit('continue-search')"
      @retry-search="$emit('retry-search')"
      @restart-search="$emit('restart-search')"
    />
  </article>
</template>

<style scoped>
.result-card { min-width: 0; display: grid; grid-template-columns: minmax(0, 1fr) 12.5rem; gap: .5rem .75rem; align-items: center; padding: .8rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.main { grid-column: 1; grid-row: 1 / span 2; min-width: 0; align-self: stretch; display: grid; grid-template-columns: 3.5rem minmax(0, 1fr); gap: .8rem; align-items: center; border: 0; border-radius: var(--radius-md); padding: 0; background: transparent; color: inherit; text-align: left; cursor: pointer; }
.preview-action { grid-column: 2; grid-row: 1; width: 100%; align-self: end; }
.cover { width: 3.5rem; border-radius: var(--radius-sm); }
.info { min-width: 0; display: grid; gap: .2rem; overflow-wrap: anywhere; color: var(--color-ink-muted); font-size: .82rem; }.info strong { overflow: hidden; color: var(--color-ink); font: 700 1rem var(--font-literary); text-overflow: ellipsis; white-space: nowrap; }.chapter { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.source { min-width: 0; color: var(--color-accent); font-size: .75rem; }
.main:hover .info strong { color: var(--color-accent-strong); }
.main:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 3px; }
@media (max-width: 35rem) { .result-card { grid-template-columns: 1fr; }.main { grid-column: 1; grid-row: 1; grid-template-columns: 3.25rem minmax(0, 1fr); }.preview-action { grid-column: 1; grid-row: 2; align-self: auto; } }
</style>
