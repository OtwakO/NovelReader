<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { SearchResult } from '../../../api/search';
import AppButton from '../../../ui/components/AppButton.vue';

export default defineComponent({
  name: 'SearchResultCard', components: { AppButton }, props: { result: { type: Object as PropType<SearchResult>, required: true }, shelving: Boolean }, emits: ['open', 'shelve'],
  computed: { sourceCount(): number { return 1 + (this.result.alternateSources?.length ?? 0); } },
});
</script>

<template>
  <article class="result-card">
    <button type="button" class="main" :aria-label="$t('search.results.detailsFor', { name: result.name })" @click="$emit('open')">
      <img v-if="result.coverUrl" :src="result.coverUrl" alt="" class="cover" loading="lazy">
      <span v-else class="cover placeholder" aria-hidden="true">{{ result.name.slice(0, 1) }}</span>
      <span class="info"><strong>{{ result.name }}</strong><span>{{ result.author || $t('app.common.unknownAuthor') }}</span><span v-if="result.lastChapter" class="chapter">{{ result.lastChapter }}</span><span class="source">{{ result.sourceName }} · {{ $t('search.results.sources', { count: sourceCount }) }}</span></span>
      <span class="details">{{ $t('search.results.details') }}</span>
    </button>
    <AppButton variant="secondary" :busy="shelving" @click="$emit('shelve')">{{ shelving ? $t('search.results.shelving') : $t('search.results.shelve') }}</AppButton>
  </article>
</template>

<style scoped>
.result-card { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: .75rem; align-items: center; padding: .8rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.main { min-width: 0; display: grid; grid-template-columns: 3.5rem minmax(0, 1fr) auto; gap: .8rem; align-items: center; border: 0; padding: 0; background: transparent; color: inherit; text-align: left; cursor: pointer; }
.cover { width: 3.5rem; aspect-ratio: 2 / 3; object-fit: cover; border-radius: var(--radius-sm); }.placeholder { display: grid; place-items: center; background: linear-gradient(145deg, var(--color-accent), var(--color-accent-strong)); color: white; font: 700 1.3rem var(--font-literary); }
.info { min-width: 0; display: grid; gap: .2rem; color: var(--color-ink-muted); font-size: .82rem; }.info strong { overflow: hidden; color: var(--color-ink); font: 700 1rem var(--font-literary); text-overflow: ellipsis; white-space: nowrap; }.chapter { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.source { color: var(--color-accent); font-size: .75rem; }.details { min-height: 2.5rem; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .5rem .8rem; background: var(--color-paper); color: var(--color-ink); font-size: .82rem; font-weight: 700; }
.main:hover .details { border-color: color-mix(in srgb, var(--color-accent) 55%, var(--color-border)); background: var(--color-paper-raised); }
.main:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 3px; }
@media (max-width: 35rem) { .result-card { grid-template-columns: 1fr; }.main { grid-template-columns: 3.25rem minmax(0, 1fr); }.details { grid-column: 1 / -1; width: 100%; }.result-card :deep(.app-button) { width: 100%; } }
</style>
