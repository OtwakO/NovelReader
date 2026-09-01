<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { AltSource, Book, SearchResult } from '../../api/models';
import { searchBooksBatchStream, searchInstalledSource } from '../../api/search';
import AppButton from '../../ui/components/AppButton.vue';
import SearchControls from '../search/components/SearchControls.vue';
import SearchStatus from '../search/components/SearchStatus.vue';
import { loadSearchPreferences, requestedConcurrency, saveSearchPreferences, type SearchIntensity } from '../search/search-preferences';
import { matchesLogicalBook } from '../books/book-identity';
import { createSourceDiscovery, type SourceDiscoveryState } from './source-discovery';

const emptyState: SourceDiscoveryState = { searching: false, checked: 0, eligible: 0, resultCount: 0, effectiveConcurrency: 0, sourceFailures: 0, errorCode: '', errorDetail: '', restartRequired: false, retryRequired: false, hasMore: false };

type KnownSource = { binding: AltSource; current: boolean; stored: boolean };

function bindingKey(source: Pick<AltSource, 'sourceId' | 'bookUrl'>): string {
  return `${source.sourceId}\n${source.bookUrl}`;
}

function sameBindingMetadata(left: AltSource, right: AltSource): boolean {
  return left.sourceId === right.sourceId
    && left.sourceUrl === right.sourceUrl
    && left.bookUrl === right.bookUrl
    && left.sourceName === right.sourceName
    && left.sourceGroup === right.sourceGroup
    && left.discoveryQuery === right.discoveryQuery
    && left.lastChapter === right.lastChapter
    && (left.capabilities ?? []).join('\n') === (right.capabilities ?? []).join('\n');
}

function mergeBinding(existing: AltSource | undefined, incoming: AltSource): AltSource {
  return {
    sourceId: incoming.sourceId,
    sourceUrl: incoming.sourceUrl || existing?.sourceUrl || '',
    bookUrl: incoming.bookUrl,
    sourceName: incoming.sourceName || existing?.sourceName || '',
    sourceGroup: incoming.sourceGroup || existing?.sourceGroup,
    capabilities: incoming.capabilities?.length ? [...incoming.capabilities] : existing?.capabilities,
    discoveryQuery: incoming.discoveryQuery || existing?.discoveryQuery,
    lastChapter: incoming.lastChapter || existing?.lastChapter,
  };
}

export default defineComponent({
  name: 'SourceRecoveryPanel',
  components: { AppButton, SearchControls, SearchStatus },
  props: {
    book: { type: Object as PropType<Book>, required: true },
    switching: Boolean,
    actionError: { type: String, default: '' },
    actionMessage: { type: String, default: '' },
    onClearAndRescan: { type: Function as PropType<() => Promise<void>>, required: true },
  },
  emits: ['select', 'matches'],
  data() {
    const preferences = loadSearchPreferences();
    return { batchSize: preferences.batchSize, intensity: preferences.intensity as SearchIntensity, advancedConcurrency: preferences.advancedConcurrency, matches: [] as AltSource[], state: { ...emptyState }, controller: null as ReturnType<typeof createSourceDiscovery> | null, rescanning: false, confirmingRescan: false, filterQuery: '', filterKind: 'all' as 'all'|'stored'|'new', targetedQuery: '', targetedSearching: false, targetedError: '', targetedMessage: '' };
  },
  computed: {
    activeBinding(): AltSource {
      return this.book.activeSource || { sourceId: this.book.sourceId, sourceUrl: this.book.sourceUrl, bookUrl: this.book.bookUrl, sourceName: this.book.origin || this.book.sourceUrl, lastChapter: this.book.lastChapter };
    },
    sources(): KnownSource[] {
      const activeKey = bindingKey(this.activeBinding);
      const stored = this.book.alternateSources ?? [];
      const storedKeys = new Set([activeKey, ...stored.map(bindingKey)]);
      const bindings = new Map<string, AltSource>();
      for (const source of [this.activeBinding, ...stored, ...this.matches]) {
        const key = bindingKey(source);
        bindings.set(key, mergeBinding(bindings.get(key), source));
      }
      return [...bindings.entries()].map(([key, binding]) => ({ binding, current: key === activeKey, stored: storedKeys.has(key) }));
    },
    knownSourceCount(): number { return this.sources.length; },
    visibleSources(): KnownSource[] {
      const query = this.filterQuery.trim().toLocaleLowerCase();
      return this.sources.filter(source => {
        if (this.filterKind === 'stored' && !source.stored) return false;
        if (this.filterKind === 'new' && source.stored) return false;
        return !query || `${source.binding.sourceName}\n${source.binding.sourceUrl}\n${source.binding.lastChapter || ''}`.toLocaleLowerCase().includes(query);
      });
    },
    filtering(): boolean { return Boolean(this.filterQuery.trim()) || this.filterKind !== 'all'; },
    moreCount(): number { return Math.min(this.batchSize, Math.max(0, this.state.eligible - this.state.checked)); },
  },
  watch: {
    'book.name'() { this.resetDiscovery(); },
    'book.author'() { this.resetDiscovery(); },
  },
  created() {
    this.controller = createSourceDiscovery({
      query: () => this.book.name,
      preferences: () => ({ batchSize: this.batchSize, concurrency: requestedConcurrency({ batchSize: this.batchSize, intensity: this.intensity, advancedConcurrency: this.advancedConcurrency }) }),
      openStream: searchBooksBatchStream,
      onChange: (state) => { this.state = state; },
      onResults: (items) => this.acceptResults(items),
    });
  },
  beforeUnmount() { this.controller?.destroy(); },
  methods: {
    key(source: AltSource) { return bindingKey(source); },
    acceptResults(items: SearchResult[], discoveryQuery = '') {
      const found: AltSource[] = [];
      for (const item of items) {
        if (!matchesLogicalBook(this.book, item)) continue;
        for (const source of [{ sourceId: item.sourceId, sourceUrl: item.sourceUrl, bookUrl: item.bookUrl, sourceName: item.sourceName, sourceGroup: item.sourceGroup, capabilities: item.capabilities, discoveryQuery, lastChapter: item.lastChapter }, ...(item.alternateSources ?? []).map(source => ({ ...source, discoveryQuery }))]) {
          const key = this.key(source);
          const existing = this.sources.find(item => this.key(item.binding) === key)?.binding;
          const merged = mergeBinding(existing, source);
          const discovered = this.matches.findIndex(item => this.key(item) === key);
          if (discovered >= 0) this.matches[discovered] = merged;
          else this.matches.push(merged);
          if (!existing || !sameBindingMetadata(existing, merged)) found.push(merged);
        }
      }
      if (found.length) this.$emit('matches', found);
      return found.length;
    },
    savePreferences() { saveSearchPreferences({ batchSize: this.batchSize, intensity: this.intensity, advancedConcurrency: this.advancedConcurrency }); },
    resetDiscovery() { this.controller?.destroy(); this.matches = []; this.state = { ...emptyState }; },
    async rescan() {
      this.controller?.stop(); this.rescanning = true; this.confirmingRescan = false;
      try { await this.onClearAndRescan(); this.matches = []; this.controller?.restart(); }
      finally { this.rescanning = false; }
    },
    async searchCurrentSource() {
      if (!this.targetedQuery.trim() || this.targetedSearching) return;
      const query = this.targetedQuery;
      this.targetedSearching = true; this.targetedError = ''; this.targetedMessage = '';
      try {
        const saved = this.acceptResults(await searchInstalledSource(this.activeBinding.sourceId, query), query);
        this.targetedMessage = saved ? this.$t('sourceRecovery.targetedSaved', { count: saved }) : this.$t('sourceRecovery.targetedEmpty');
      } catch (cause) {
        this.targetedError = cause instanceof Error ? cause.message : this.$t('sourceRecovery.targetedFailed');
      } finally {
        this.targetedSearching = false;
      }
    },
  },
});
</script>

<template>
  <section class="recovery" aria-labelledby="source-recovery-heading">
    <header><div><p class="eyebrow">{{ $t('sourceRecovery.eyebrow') }}</p><h2 id="source-recovery-heading">{{ $t('sourceRecovery.title') }}</h2><p>{{ $t('sourceRecovery.description') }}</p></div><div class="header-actions"><AppButton v-if="state.searching" variant="secondary" @click="controller?.stop()">{{ $t('sourceRecovery.stop') }}</AppButton><AppButton variant="quiet" :busy="rescanning" @click="confirmingRescan = true">{{ rescanning ? $t('sourceRecovery.clearing') : $t('sourceRecovery.rescan') }}</AppButton></div></header>
    <section v-if="confirmingRescan" class="confirmation" role="alertdialog" :aria-label="$t('sourceRecovery.confirmTitle')"><strong>{{ $t('sourceRecovery.confirmTitle') }}</strong><p>{{ $t('sourceRecovery.confirmDescription') }}</p><div><AppButton variant="secondary" @click="confirmingRescan = false">{{ $t('sourceRecovery.cancel') }}</AppButton><AppButton variant="danger" @click="rescan">{{ $t('sourceRecovery.confirm') }}</AppButton></div></section>
    <SearchControls v-model:batch-size="batchSize" v-model:intensity="intensity" v-model:advanced-concurrency="advancedConcurrency" @change="savePreferences" />
    <SearchStatus v-if="state.checked || state.searching || state.errorCode" :checked="state.checked" :eligible="state.eligible" :result-count="knownSourceCount" result-label-key="sourceRecovery.knownSources" :searching="state.searching" :concurrency="state.effectiveConcurrency" :source-failures="state.sourceFailures" :error-code="state.errorCode" :error-detail="state.errorDetail" :storage-warning="false" :restart-required="state.restartRequired" :retry-required="state.retryRequired" :has-more="state.hasMore" :more-count="moreCount" @restart="controller?.restart()" @retry="controller?.retry()" @more="controller?.more()" />
    <p v-if="actionError" class="error" role="alert">{{ actionError }}</p><p v-else-if="actionMessage" class="message" role="status">{{ actionMessage }}</p>
    <details class="targeted-search"><summary>{{ $t('sourceRecovery.targetedTitle') }}</summary><p>{{ $t('sourceRecovery.targetedDescription') }}</p><form @submit.prevent="searchCurrentSource"><label><span>{{ $t('sourceRecovery.targetedLabel') }}</span><input v-model="targetedQuery" type="search" :placeholder="$t('sourceRecovery.targetedPlaceholder')" autocomplete="off"></label><AppButton type="submit" variant="secondary" :busy="targetedSearching" :disabled="!targetedQuery.trim()">{{ $t('sourceRecovery.targetedAction') }}</AppButton></form><p v-if="targetedError" class="error" role="alert">{{ targetedError }}</p><p v-else-if="targetedMessage" class="message" role="status">{{ targetedMessage }}</p></details>
    <div v-if="sources.length" class="source-filter"><label><span>{{ $t('sourceRecovery.filterLabel') }}</span><input v-model="filterQuery" type="search" :placeholder="$t('sourceRecovery.filterPlaceholder')"></label><div role="group" :aria-label="$t('sourceRecovery.filterKinds')"><button type="button" :aria-pressed="filterKind==='all'" @click="filterKind='all'">{{ $t('sourceRecovery.all') }}</button><button type="button" :aria-pressed="filterKind==='stored'" @click="filterKind='stored'">{{ $t('sourceRecovery.stored') }}</button><button type="button" :aria-pressed="filterKind==='new'" @click="filterKind='new'">{{ $t('sourceRecovery.new') }}</button></div></div>
    <ul v-if="visibleSources.length" class="sources"><li v-for="source in visibleSources" :key="key(source.binding)" :class="{ current: source.current }"><span class="source-content"><span class="source-heading"><strong>{{ source.binding.sourceName || source.binding.sourceUrl }}</strong><em v-if="source.current" class="current-badge">{{ $t('sourceRecovery.currentSource') }}</em></span><small v-if="source.binding.lastChapter" class="last-chapter">{{ source.binding.lastChapter }}</small><small>{{ source.binding.sourceUrl }}</small><small v-if="source.binding.discoveryQuery" class="discovery-query">{{ $t('sourceRecovery.discoveredByQuery', { query: source.binding.discoveryQuery }) }}</small><span v-if="source.binding.sourceGroup || source.binding.capabilities?.length" class="badges"><em v-if="source.binding.sourceGroup">{{ source.binding.sourceGroup }}</em><em v-for="capability in source.binding.capabilities" :key="capability">{{ $t(`sources.capabilities.${capability}`) }}</em></span></span><AppButton v-if="!source.current" variant="secondary" :disabled="switching" @click="$emit('select', source.binding)">{{ switching ? $t('sourceRecovery.switching') : $t('sourceRecovery.use') }}</AppButton></li></ul>
    <p v-else-if="filtering && sources.length" class="empty">{{ $t('sourceRecovery.noFilterMatches') }}</p>
    <p v-else-if="state.checked && !state.searching && !targetedMessage && !targetedError" class="empty">{{ $t('sourceRecovery.empty') }}</p>
    <AppButton v-if="!state.searching && state.checked === 0" @click="controller?.start()">{{ $t('sourceRecovery.find') }}</AppButton>
  </section>
</template>

<style scoped>
.recovery { display: grid; gap: .85rem; padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }.targeted-search{border:1px solid var(--color-border);border-radius:var(--radius-md);padding:.75rem}.targeted-search summary{cursor:pointer;font-weight:700}.targeted-search>p{margin:.55rem 0;color:var(--color-ink-muted);line-height:1.5}.targeted-search form{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:.65rem;align-items:end}.targeted-search label{display:grid;gap:.3rem;color:var(--color-ink-muted);font-size:.75rem;font-weight:700}.targeted-search input{width:100%;min-height:2.75rem;border:1px solid var(--color-border);border-radius:var(--radius-md);padding:.55rem .7rem;background:white;color:var(--color-ink)}.source-filter{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:.75rem;align-items:end}.source-filter label{display:grid;gap:.3rem;color:var(--color-ink-muted);font-size:.75rem;font-weight:700}.source-filter input{width:100%;min-height:2.75rem;border:1px solid var(--color-border);border-radius:var(--radius-md);padding:.55rem .7rem;background:white;color:var(--color-ink)}.source-filter div{display:flex;gap:.35rem}.source-filter button{min-height:2.75rem;border:1px solid var(--color-border);border-radius:var(--radius-md);padding:.5rem .7rem;background:var(--color-paper);color:var(--color-ink)}.source-filter button[aria-pressed=true]{border-color:var(--color-accent);background:var(--color-accent-soft);color:var(--color-accent-strong)} header { display: flex; justify-content: space-between; gap: 1rem; } h2 { margin: .15rem 0; font: 700 1.2rem var(--font-literary); } header p { margin: .25rem 0 0; color: var(--color-ink-muted); line-height: 1.55; }.eyebrow { color: var(--color-warm); font-size: .72rem; font-weight: 800; letter-spacing: .1em; text-transform: uppercase; }.header-actions { align-self: flex-start; display: flex; flex-wrap: wrap; justify-content: flex-end; align-items: center; gap: .4rem; }.header-actions :deep(.app-button) { white-space: nowrap; }.sources { list-style: none; display: grid; gap: .55rem; margin: 0; padding: 0; }.sources li { display: grid; grid-template-columns: minmax(0,1fr) auto; gap: .75rem; align-items: center; padding: .7rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); }.sources li.current{border-color:color-mix(in srgb,var(--color-accent) 35%,var(--color-border))}.source-content { min-width: 0; display: grid; gap: .2rem; }.source-heading{display:flex;flex-wrap:wrap;align-items:center;gap:.45rem}.current-badge{padding:.15rem .4rem;border-radius:var(--radius-sm);background:var(--color-accent-soft);color:var(--color-accent-strong);font-size:.68rem;font-style:normal;font-weight:800;line-height:1.35}.sources strong, .sources small { overflow-wrap: anywhere; }.sources small, .empty { color: var(--color-ink-muted); }.last-chapter{color:var(--color-ink);font-weight:650}.discovery-query{color:var(--color-accent-strong);font-weight:700}.badges{display:flex;flex-wrap:wrap;gap:.25rem;margin-top:.2rem}.badges em{border-radius:999px;padding:.18rem .42rem;background:var(--color-accent-soft);color:var(--color-accent-strong);font-size:.68rem;font-style:normal}.confirmation { padding: .85rem; border: 1px solid color-mix(in srgb, var(--color-danger) 45%, var(--color-border)); border-radius: var(--radius-md); background: #fff5f1; }.confirmation p { margin: .35rem 0 .7rem; color: var(--color-ink-muted); }.confirmation div { display: flex; flex-wrap: wrap; gap: .5rem; }.error { color: var(--color-danger); }.message { color: var(--color-success); }
@media (max-width: 36rem) { header { flex-direction: column; }.header-actions { justify-content: stretch; }.targeted-search form,.source-filter{grid-template-columns:1fr}.targeted-search :deep(.app-button){width:100%}.source-filter div{display:grid;grid-template-columns:repeat(3,1fr)}.sources li { grid-template-columns: 1fr; }.sources :deep(.app-button) { width: 100%; } }
</style>
