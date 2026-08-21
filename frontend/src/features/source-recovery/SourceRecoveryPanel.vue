<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { AltSource, SearchResult } from '../../api/models';
import { searchBooksBatchStream } from '../../api/search';
import AppButton from '../../ui/components/AppButton.vue';
import SearchControls from '../search/components/SearchControls.vue';
import SearchStatus from '../search/components/SearchStatus.vue';
import { loadSearchPreferences, requestedConcurrency, saveSearchPreferences, type SearchIntensity } from '../search/search-preferences';
import { matchesLogicalBook } from './book-identity';
import { createSourceDiscovery, type SourceDiscoveryState } from './source-discovery';

const emptyState: SourceDiscoveryState = { searching: false, checked: 0, eligible: 0, resultCount: 0, effectiveConcurrency: 0, sourceFailures: 0, errorCode: '', errorDetail: '', restartRequired: false, retryRequired: false, hasMore: false };

export default defineComponent({
  name: 'SourceRecoveryPanel',
  components: { AppButton, SearchControls, SearchStatus },
  props: {
    book: { type: Object as PropType<{ name: string; author: string }>, required: true },
    currentSourceUrl: { type: String, required: true },
    storedSources: { type: Array as PropType<AltSource[]>, default: () => [] },
    switching: Boolean,
    actionError: { type: String, default: '' },
    actionMessage: { type: String, default: '' },
    onClearAndRescan: { type: Function as PropType<() => Promise<void>>, required: true },
  },
  emits: ['select', 'matches'],
  data() {
    const preferences = loadSearchPreferences();
    return { batchSize: preferences.batchSize, intensity: preferences.intensity as SearchIntensity, advancedConcurrency: preferences.advancedConcurrency, matches: [] as AltSource[], seen: new Set<string>(), state: { ...emptyState }, controller: null as ReturnType<typeof createSourceDiscovery> | null, rescanning: false, confirmingRescan: false, filterQuery: '', filterKind: 'all' as 'all'|'stored'|'new' };
  },
  computed: {
    sources(): AltSource[] {
      const values = [...this.storedSources, ...this.matches];
      return values.filter((source, index) => source.sourceUrl !== this.currentSourceUrl && values.findIndex((item) => item.sourceUrl === source.sourceUrl && item.bookUrl === source.bookUrl) === index);
    },
    knownSourceCount(): number { return this.sources.length + (this.currentSourceUrl.trim() ? 1 : 0); },
    visibleSources(): AltSource[] {
      const stored = new Set(this.storedSources.map(source => this.key(source)));
      const query = this.filterQuery.trim().toLocaleLowerCase();
      return this.sources.filter(source => {
        if (this.filterKind === 'stored' && !stored.has(this.key(source))) return false;
        if (this.filterKind === 'new' && stored.has(this.key(source))) return false;
        return !query || `${source.sourceName}\n${source.sourceUrl}`.toLocaleLowerCase().includes(query);
      });
    },
    filtering(): boolean { return Boolean(this.filterQuery.trim()) || this.filterKind !== 'all'; },
    moreCount(): number { return Math.min(this.batchSize, Math.max(0, this.state.eligible - this.state.checked)); },
  },
  watch: {
    'book.name'() { this.resetDiscovery(); },
    'book.author'() { this.resetDiscovery(); },
    storedSources: { deep: true, handler() { this.seedStoredSources(); } },
  },
  created() {
    this.seedStoredSources();
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
    key(source: AltSource) { return `${source.sourceUrl}\n${source.bookUrl}`; },
    seedStoredSources() { for (const source of this.storedSources) this.seen.add(this.key(source)); },
    acceptResults(items: SearchResult[]) {
      const found: AltSource[] = [];
      for (const item of items) {
        if (!matchesLogicalBook(this.book, item)) continue;
        for (const source of [{ sourceUrl: item.sourceUrl, bookUrl: item.bookUrl, sourceName: item.sourceName }, ...(item.alternateSources ?? [])]) {
          if (source.sourceUrl === this.currentSourceUrl || this.seen.has(this.key(source))) continue;
          this.seen.add(this.key(source)); this.matches.push(source); found.push(source);
        }
      }
      if (found.length) this.$emit('matches', found);
    },
    savePreferences() { saveSearchPreferences({ batchSize: this.batchSize, intensity: this.intensity, advancedConcurrency: this.advancedConcurrency }); },
    resetDiscovery() { this.controller?.destroy(); this.seen = new Set(); this.matches = []; this.seedStoredSources(); this.state = { ...emptyState }; },
    async rescan() {
      this.controller?.stop(); this.rescanning = true; this.confirmingRescan = false;
      try { await this.onClearAndRescan(); this.seen = new Set(); this.matches = []; this.controller?.restart(); }
      finally { this.rescanning = false; }
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
    <div v-if="sources.length" class="source-filter"><label><span>{{ $t('sourceRecovery.filterLabel') }}</span><input v-model="filterQuery" type="search" :placeholder="$t('sourceRecovery.filterPlaceholder')"></label><div role="group" :aria-label="$t('sourceRecovery.filterKinds')"><button type="button" :aria-pressed="filterKind==='all'" @click="filterKind='all'">{{ $t('sourceRecovery.all') }}</button><button type="button" :aria-pressed="filterKind==='stored'" @click="filterKind='stored'">{{ $t('sourceRecovery.stored') }}</button><button type="button" :aria-pressed="filterKind==='new'" @click="filterKind='new'">{{ $t('sourceRecovery.new') }}</button></div></div>
    <ul v-if="visibleSources.length" class="sources"><li v-for="source in visibleSources" :key="key(source)"><span><strong>{{ source.sourceName || source.sourceUrl }}</strong><small>{{ source.sourceUrl }}</small></span><AppButton variant="secondary" :disabled="switching" @click="$emit('select', source)">{{ switching ? $t('sourceRecovery.switching') : $t('sourceRecovery.use') }}</AppButton></li></ul>
    <p v-else-if="filtering && sources.length" class="empty">{{ $t('sourceRecovery.noFilterMatches') }}</p>
    <p v-else-if="state.checked && !state.searching" class="empty">{{ $t('sourceRecovery.empty') }}</p>
    <AppButton v-if="!state.searching && state.checked === 0" @click="controller?.start()">{{ $t('sourceRecovery.find') }}</AppButton>
  </section>
</template>

<style scoped>
.recovery { display: grid; gap: .85rem; padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }.source-filter{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:.75rem;align-items:end}.source-filter label{display:grid;gap:.3rem;color:var(--color-ink-muted);font-size:.75rem;font-weight:700}.source-filter input{width:100%;min-height:2.75rem;border:1px solid var(--color-border);border-radius:var(--radius-md);padding:.55rem .7rem;background:white;color:var(--color-ink)}.source-filter div{display:flex;gap:.35rem}.source-filter button{min-height:2.75rem;border:1px solid var(--color-border);border-radius:var(--radius-md);padding:.5rem .7rem;background:var(--color-paper);color:var(--color-ink)}.source-filter button[aria-pressed=true]{border-color:var(--color-accent);background:var(--color-accent-soft);color:var(--color-accent-strong)} header { display: flex; justify-content: space-between; gap: 1rem; } h2 { margin: .15rem 0; font: 700 1.2rem var(--font-literary); } header p { margin: .25rem 0 0; color: var(--color-ink-muted); line-height: 1.55; }.eyebrow { color: var(--color-warm); font-size: .72rem; font-weight: 800; letter-spacing: .1em; text-transform: uppercase; }.header-actions { align-self: flex-start; display: flex; flex-wrap: wrap; justify-content: flex-end; align-items: center; gap: .4rem; }.header-actions :deep(.app-button) { white-space: nowrap; }.sources { list-style: none; display: grid; gap: .55rem; margin: 0; padding: 0; }.sources li { display: grid; grid-template-columns: minmax(0,1fr) auto; gap: .75rem; align-items: center; padding: .7rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); }.sources span { min-width: 0; display: grid; gap: .2rem; }.sources strong, .sources small { overflow-wrap: anywhere; }.sources small, .empty { color: var(--color-ink-muted); }.confirmation { padding: .85rem; border: 1px solid color-mix(in srgb, var(--color-danger) 45%, var(--color-border)); border-radius: var(--radius-md); background: #fff5f1; }.confirmation p { margin: .35rem 0 .7rem; color: var(--color-ink-muted); }.confirmation div { display: flex; flex-wrap: wrap; gap: .5rem; }.error { color: var(--color-danger); }.message { color: var(--color-success); }
@media (max-width: 36rem) { header { flex-direction: column; }.header-actions { justify-content: stretch; }.source-filter{grid-template-columns:1fr}.source-filter div{display:grid;grid-template-columns:repeat(3,1fr)}.sources li { grid-template-columns: 1fr; }.sources :deep(.app-button) { width: 100%; } }
</style>
