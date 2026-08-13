import { defineStore } from 'pinia';
import { searchBooksBatchStream, type SearchResult } from '../../api/search';
import { loadSearchPreferences, requestedConcurrency, saveSearchPreferences, type SearchIntensity } from './search-preferences';
import { mergeSearchResults } from './search-results';

const storageKey = 'novelreader.search-session';
const stateVersion = 1;

interface RestoredState {
  version: number; query: string; searchedQuery: string; results: SearchResult[]; checked: number; eligible: number;
  committedOffset: number; cursor: string; retryCursor: string; hasMore: boolean; retryRequired: boolean; restartRequired: boolean;
  batchSourceIds: string[]; sourceFailures: number; activeBatchSize: number; activeConcurrency: number; inFlight: boolean;
}

export const useSearchStore = defineStore('search', {
  state: () => ({
    query: '', searchedQuery: '', results: [] as SearchResult[], searching: false, checked: 0, eligible: 0, committedOffset: 0,
    cursor: '', retryCursor: '', hasMore: false, retryRequired: false, restartRequired: false, batchSourceIds: [] as string[],
    sourceFailures: 0, effectiveConcurrency: 0, activeBatchSize: 50, activeConcurrency: 8, errorCode: '' as '' | 'stale' | 'disconnect',
    errorDetail: '', storageWarning: false, batchSize: 50, intensity: 'balanced' as SearchIntensity, advancedConcurrency: 8,
    initialized: false, stream: null as EventSource | null, generation: 0,
  }),
  getters: {
    resultCount: (state) => state.results.length,
    multipleSourceCount: (state) => state.results.filter((result) => result.alternateSources?.length).length,
    progressPercent: (state) => state.eligible > 0 ? Math.min(100, Math.round((state.checked / state.eligible) * 100)) : 0,
    moreCount: (state) => Math.min(state.batchSize, Math.max(0, state.eligible - state.checked)),
  },
  actions: {
    initialize() {
      if (this.initialized) return;
      this.initialized = true;
      Object.assign(this, loadSearchPreferences());
      try {
        const saved = JSON.parse(sessionStorage.getItem(storageKey) || 'null') as RestoredState | null;
        if (!saved || saved.version !== stateVersion || !Array.isArray(saved.results) || !Array.isArray(saved.batchSourceIds)) return;
        this.query = saved.query || ''; this.searchedQuery = saved.searchedQuery || this.query; this.results = saved.results;
        this.checked = Math.max(0, saved.checked); this.eligible = Math.max(0, saved.eligible); this.committedOffset = Math.max(0, saved.committedOffset);
        this.cursor = saved.cursor || ''; this.retryCursor = saved.retryCursor || ''; this.hasMore = Boolean(saved.hasMore);
        this.retryRequired = Boolean(saved.retryRequired || saved.inFlight); this.restartRequired = Boolean(saved.restartRequired);
        this.batchSourceIds = saved.batchSourceIds.filter((value) => typeof value === 'string'); this.sourceFailures = Math.max(0, saved.sourceFailures);
        this.activeBatchSize = Math.min(500, Math.max(1, saved.activeBatchSize)); this.activeConcurrency = Math.max(1, saved.activeConcurrency);
      } catch { sessionStorage.removeItem(storageKey); }
    },
    persistPreferences() {
      this.batchSize = Math.min(500, Math.max(1, Math.trunc(this.batchSize || 1)));
      this.advancedConcurrency = Math.max(1, Math.trunc(this.advancedConcurrency || 1));
      saveSearchPreferences({ batchSize: this.batchSize, intensity: this.intensity, advancedConcurrency: this.advancedConcurrency });
    },
    save() {
      try {
        sessionStorage.setItem(storageKey, JSON.stringify({ version: stateVersion, query: this.query, searchedQuery: this.searchedQuery, results: this.results, checked: this.checked, eligible: this.eligible, committedOffset: this.committedOffset, cursor: this.cursor, retryCursor: this.retryCursor, hasMore: this.hasMore, retryRequired: this.retryRequired, restartRequired: this.restartRequired, batchSourceIds: this.batchSourceIds, sourceFailures: this.sourceFailures, activeBatchSize: this.activeBatchSize, activeConcurrency: this.activeConcurrency, inFlight: this.searching }));
        this.storageWarning = false;
      } catch { this.storageWarning = true; }
    },
    resetFor(query: string) {
      this.stop(false); this.searchedQuery = query; this.results = []; this.checked = 0; this.eligible = 0; this.committedOffset = 0;
      this.cursor = ''; this.retryCursor = ''; this.hasMore = false; this.retryRequired = false; this.restartRequired = false;
      this.batchSourceIds = []; this.sourceFailures = 0; this.errorCode = ''; this.errorDetail = ''; sessionStorage.removeItem(storageKey);
    },
    search() {
      const query = this.query.trim();
      if (!query) return;
      this.resetFor(query);
      this.startBatch('', this.batchSize, requestedConcurrency(this), false);
    },
    startBatch(cursor: string, size: number, concurrency: number, preservePartial: boolean) {
      this.stop(false);
      if (!preservePartial) this.batchSourceIds = [];
      this.activeBatchSize = size; this.activeConcurrency = concurrency; this.retryCursor = cursor; this.searching = true;
      this.retryRequired = false; this.restartRequired = false; this.errorCode = ''; this.errorDetail = '';
      const generation = ++this.generation;
      this.stream = searchBooksBatchStream(this.searchedQuery, { cursor, batchSize: size, concurrency }, {
        onStart: (event) => { if (generation !== this.generation) return; this.committedOffset = event.offset; this.eligible = event.eligible; this.retryCursor = event.retryCursor; this.effectiveConcurrency = event.effectiveConcurrency; this.checked = this.committedOffset + this.batchSourceIds.length; this.save(); },
        onResult: (sourceId, items) => { if (generation !== this.generation) return; this.markSourceChecked(sourceId); this.results = mergeSearchResults(this.results, items, this.searchedQuery); this.save(); },
        onSourceError: (sourceId) => { if (generation !== this.generation) return; this.markSourceChecked(sourceId); this.save(); },
        onDone: (event) => { if (generation !== this.generation) return; this.searching = false; this.stream = null; if (event.complete) { this.committedOffset = event.checked; this.checked = event.checked; this.cursor = event.nextCursor || ''; this.retryCursor = ''; this.hasMore = event.hasMore; this.retryRequired = false; this.batchSourceIds = []; this.sourceFailures += event.sourceFailures; } else this.retryRequired = true; this.save(); },
        onStale: (message) => { if (generation !== this.generation) return; this.searching = false; this.stream = null; this.restartRequired = true; this.errorCode = 'stale'; this.errorDetail = message; this.save(); },
        onDisconnect: () => { if (generation !== this.generation) return; this.searching = false; this.stream = null; this.retryRequired = true; this.errorCode = 'disconnect'; this.save(); },
      });
      this.save();
    },
    markSourceChecked(sourceId: string) { if (!this.batchSourceIds.includes(sourceId)) this.batchSourceIds.push(sourceId); this.checked = this.committedOffset + this.batchSourceIds.length; },
    stop(markRetry = true) { this.generation += 1; this.stream?.close(); this.stream = null; if (this.searching && markRetry) this.retryRequired = true; this.searching = false; if (markRetry) this.save(); },
    retry() { this.startBatch(this.retryCursor, this.activeBatchSize, this.activeConcurrency, true); },
    more() { this.startBatch(this.cursor, this.batchSize, requestedConcurrency(this), false); },
    restart() { this.query = this.searchedQuery || this.query; this.search(); },
  },
});
