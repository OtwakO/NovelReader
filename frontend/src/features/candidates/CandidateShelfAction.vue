<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { SearchResult } from '../../api/search';
import AppButton from '../../ui/components/AppButton.vue';
import CandidateProgressDetails from './CandidateProgressDetails.vue';
import {
  cancelCandidateOperation,
  candidateBindingSignature,
  candidateOperationMatches,
  candidateWasCommitted,
  forgetCandidateOperation,
  getCandidateOperation,
  commitCandidateOperation,
  recalledCandidateOperation,
  rememberCandidateCommitted,
  rememberCandidateOperation,
  startCandidateOperation,
  subscribeCandidateOperation,
  type CandidateOperationSnapshot,
} from './candidate-operation';

const terminalFailures = ['exhausted', 'failed'] as const;

export default defineComponent({
  name: 'CandidateShelfAction',
  components: { AppButton, CandidateProgressDetails },
  props: {
    result: { type: Object as PropType<SearchResult>, required: true },
    canContinueSearch: { type: Boolean, default: false },
    continueSearchCount: { type: Number, default: 0 },
    searchScanning: { type: Boolean, default: false },
    searchRetryRequired: { type: Boolean, default: false },
    searchRestartRequired: { type: Boolean, default: false },
  },
  emits: ['continue-search', 'retry-search', 'restart-search'],
  data() {
    return {
      snapshot: null as CandidateOperationSnapshot | null,
      stream: null as EventSource | null,
      starting: false,
      cancelling: false,
      expanded: false,
      error: '',
      announcement: '',
      notice: '',
      completed: false,
      continuingSearch: false,
      observedSearchStart: false,
    };
  },
  computed: {
    bindingSignature(): string {
      return candidateBindingSignature(this.result);
    },
    running(): boolean {
      return this.starting || this.snapshot?.state === 'running' || (this.snapshot?.state === 'verified' && Boolean(this.snapshot.automaticCommit));
    },
    readyToCommit(): boolean {
      return this.snapshot?.state === 'verified' && !this.snapshot.automaticCommit;
    },
    failed(): boolean {
      return Boolean(this.snapshot && terminalFailures.includes(this.snapshot.state as typeof terminalFailures[number]));
    },
    commitPending(): boolean {
      return Boolean(this.snapshot?.commitPending);
    },
    winnerFound(): boolean {
      return Boolean((this.snapshot?.attempts ?? []).find((attempt) => attempt.state === 'verified'));
    },
    searchPhase(): 'scanning' | 'retry' | 'restart' | 'idle' {
      if (this.searchScanning) return 'scanning';
      if (this.searchRestartRequired) return 'restart';
      if (this.searchRetryRequired) return 'retry';
      return 'idle';
    },
    summary(): string {
      if (this.starting) return this.$t('candidate.starting');
      if (!this.snapshot) return '';
      if (this.snapshot.state === 'verified' || this.winnerFound) return this.$t('candidate.finishing');
      if (this.failed) return this.$t('candidate.failedSummary', { completed: this.snapshot.completed, known: this.snapshot.known });
      return this.$t('candidate.runningSummary', {
        active: this.snapshot.active,
        completed: this.snapshot.completed,
        known: this.snapshot.known,
      });
    },
  },
  watch: {
    bindingSignature() {
      this.discardStaleSnapshot();
    },
    searchPhase(value: string) {
      if (!this.continuingSearch) return;
      if (value === 'scanning') {
        this.observedSearchStart = true;
        return;
      }
      if (value === 'idle' && this.observedSearchStart) {
        this.continuingSearch = false;
        this.observedSearchStart = false;
      }
    },
  },
  async mounted() {
    if (candidateWasCommitted(this.result)) { this.completed = true; return; }
    const id = recalledCandidateOperation(this.result);
    if (!id) return;
    try {
      const snapshot = await getCandidateOperation(id);
      if (!candidateOperationMatches(this.result, snapshot)) {
        if (this.followsUpdates(snapshot)) await cancelCandidateOperation(id).catch(() => undefined);
        forgetCandidateOperation(this.result);
        return;
      }
      this.accept(snapshot);
      if (this.followsUpdates(snapshot)) this.connect(id);
    } catch {
      forgetCandidateOperation(this.result);
    }
  },
  beforeUnmount() { this.stream?.close(); },
  methods: {
    discardStaleSnapshot() {
      if (!this.snapshot || this.snapshot.state === 'committed' || this.commitPending || this.snapshot.automaticCommit) return;
      if (candidateOperationMatches(this.result, this.snapshot)) return;
      if (this.followsUpdates(this.snapshot)) void cancelCandidateOperation(this.snapshot.id).catch(() => undefined);
      forgetCandidateOperation(this.result);
      this.stream?.close();
      this.snapshot = null;
      this.expanded = false;
      this.error = '';
      this.notice = '';
    },
    async shelve() {
      if (this.readyToCommit) {
        await this.commitVerified();
        return;
      }
      await this.start();
    },
    async start() {
      if (this.running) return;
      forgetCandidateOperation(this.result);
      this.snapshot = null;
      this.error = '';
      this.announcement = '';
      this.notice = '';
      this.completed = false;
      this.expanded = true;
      this.starting = true;
      try {
        const snapshot = await startCandidateOperation(this.result, this.newBookID());
        rememberCandidateOperation(this.result, snapshot.id);
        this.accept(snapshot);
        if (this.followsUpdates(snapshot)) this.connect(snapshot.id);
      } catch (cause) {
        this.expanded = false;
        this.error = cause instanceof Error ? cause.message : this.$t('search.results.addFailed');
      } finally {
        this.starting = false;
      }
    },
    async commitVerified() {
      if (!this.snapshot || !this.readyToCommit || this.starting) return;
      this.starting = true;
      this.error = '';
      try {
        this.accept(await commitCandidateOperation(this.snapshot.id, this.newBookID()));
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t('search.results.addFailed');
      } finally {
        this.starting = false;
      }
    },
    continueSearch() {
      if (!this.snapshot || !this.failed || !this.canContinueSearch || this.searchPhase !== 'idle') return;
      forgetCandidateOperation(this.result);
      this.stream?.close();
      this.snapshot = null;
      this.expanded = false;
      this.error = '';
      this.notice = '';
      this.continuingSearch = true;
      this.observedSearchStart = false;
      this.$emit('continue-search');
    },
    retrySearch() {
      if (!this.continuingSearch || this.searchPhase !== 'retry') return;
      this.$emit('retry-search');
    },
    restartSearch() {
      if (!this.continuingSearch || this.searchPhase !== 'restart') return;
      this.$emit('restart-search');
    },
    async retry() {
      if (!this.snapshot || this.running) return;
      if (!this.commitPending) {
        await this.start();
        return;
      }
      this.starting = true;
      this.error = '';
      try {
        this.accept(await commitCandidateOperation(this.snapshot.id, ''));
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t('search.results.addFailed');
      } finally {
        this.starting = false;
      }
    },
    async cancel() {
      if (!this.snapshot || this.cancelling) return;
      this.cancelling = true;
      this.error = '';
      try {
        await cancelCandidateOperation(this.snapshot.id);
      } catch (cause) {
        this.cancelling = false;
        this.error = cause instanceof Error ? cause.message : this.$t('candidate.cancelFailed');
      }
    },
    connect(id: string) {
      this.stream?.close();
      this.stream = subscribeCandidateOperation(id, {
        onSnapshot: (snapshot) => this.accept(snapshot),
        onDisconnect: () => { this.error = this.$t('search.results.disconnected'); },
      });
    },
    accept(snapshot: CandidateOperationSnapshot) {
      this.error = '';
      if (snapshot.state === 'committed') {
        this.announcement = this.$t('search.results.added');
        this.notice = this.$t('search.results.added');
        this.completed = true;
        if (snapshot.storedBook?.id) rememberCandidateCommitted(this.result, snapshot.storedBook.id);
        else forgetCandidateOperation(this.result);
        this.finishAndReset(false);
        return;
      }
      if (snapshot.state === 'cancelled') {
        this.announcement = this.$t('search.results.cancelled');
        this.notice = this.$t('search.results.cancelled');
        this.finishAndReset(true);
        return;
      }
      this.snapshot = snapshot;
      this.cancelling = false;
      if (this.failed) this.expanded = true;
      if (this.isTerminal(snapshot)) this.stream?.close();
    },
    finishAndReset(forget: boolean) {
      this.stream?.close();
      if (forget) forgetCandidateOperation(this.result);
      this.snapshot = null;
      this.starting = false;
      this.cancelling = false;
      this.expanded = false;
    },
    followsUpdates(snapshot: CandidateOperationSnapshot) {
      return snapshot.state === 'running' || (snapshot.state === 'verified' && Boolean(snapshot.automaticCommit));
    },
    isTerminal(snapshot: CandidateOperationSnapshot) {
      return ['committed', 'exhausted', 'cancelled', 'failed'].includes(snapshot.state);
    },
    newBookID() {
      return crypto.randomUUID?.() ?? `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`;
    },
  },
});
</script>

<template>
  <div class="candidate-action">
    <div v-if="completed" class="completed-status" role="status">
      <svg viewBox="0 0 20 20" aria-hidden="true"><path d="m4.5 10.5 3.25 3.25L15.5 6" /></svg>
      <span>{{ $t('search.results.added') }}</span>
    </div>
    <AppButton v-else-if="continuingSearch && searchPhase === 'retry'" variant="secondary" @click="retrySearch">{{ $t('search.actions.retry') }}</AppButton>
    <AppButton v-else-if="continuingSearch && searchPhase === 'restart'" variant="secondary" @click="restartSearch">{{ $t('search.actions.restart') }}</AppButton>
    <AppButton v-else-if="continuingSearch" variant="secondary" busy>{{ $t('search.actions.more', { count: continueSearchCount }) }}</AppButton>
    <AppButton v-else-if="failed && canContinueSearch && !commitPending" variant="secondary" :disabled="searchPhase !== 'idle'" @click="continueSearch">{{ $t('search.actions.more', { count: continueSearchCount }) }}</AppButton>
    <AppButton v-else-if="failed" variant="secondary" :busy="starting" @click="retry">{{ commitPending ? $t('search.results.retryCommit') : $t('search.results.retryAdd') }}</AppButton>
    <AppButton v-else-if="(!snapshot || readyToCommit) && !starting" variant="secondary" @click="shelve">{{ $t('search.results.shelve') }}</AppButton>
    <button
      v-else
      type="button"
      class="progress-toggle"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <span>{{ summary }}</span>
      <svg viewBox="0 0 20 20" aria-hidden="true" :class="{ expanded }"><path d="m6 8 4 4 4-4" /></svg>
    </button>

    <button
      v-if="failed"
      type="button"
      class="failure-summary"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <span>{{ summary }}</span>
      <svg viewBox="0 0 20 20" aria-hidden="true" :class="{ expanded }"><path d="m6 8 4 4 4-4" /></svg>
    </button>

    <section v-if="snapshot && expanded" class="progress-detail" :class="{ 'after-failure': failed }" :aria-label="$t('candidate.title')">
      <header>
        <div>
          <strong>{{ $t('candidate.title') }}</strong>
          <span>{{ $t('candidate.help') }}</span>
        </div>
        <AppButton v-if="running" variant="quiet" :busy="cancelling" @click="cancel">
          {{ cancelling ? $t('candidate.cancelling') : $t('search.results.cancelShelving') }}
        </AppButton>
      </header>

      <CandidateProgressDetails :snapshot="snapshot" />
      <p v-if="error" class="failure" role="alert">{{ error }}</p>
    </section>

    <p v-if="error && !expanded" class="failure compact-error" :class="{ 'after-failure': failed }" role="alert">{{ error }}</p>
    <p v-if="notice && !completed" class="notice" role="status">{{ notice }}</p>
    <p class="sr-only" role="status" aria-live="polite">{{ announcement }}</p>
  </div>
</template>

<style scoped>
.candidate-action { display: contents; }
.candidate-action > :deep(.app-button),.progress-toggle,.completed-status { grid-column: 2; grid-row: 2; width: 100%; align-self: start; }
.progress-toggle,.failure-summary { min-height: 2.75rem; display: grid; grid-template-columns: minmax(0, 1fr) 1rem; align-items: center; gap: .45rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .55rem .7rem; background: var(--color-paper); color: var(--color-ink); font: inherit; font-size: .78rem; font-weight: 700; text-align: left; cursor: pointer; }
.progress-toggle:hover,.failure-summary:hover { border-color: color-mix(in srgb, var(--color-accent) 55%, var(--color-border)); }
.failure-summary { grid-column: 1 / -1; grid-row: 3; color: var(--color-danger); }
.completed-status { min-height: 2.75rem; display: flex; align-items: center; justify-content: center; gap: .45rem; border: 1px solid color-mix(in srgb, var(--color-success) 45%, var(--color-border)); border-radius: var(--radius-md); padding: .55rem .7rem; background: color-mix(in srgb, var(--color-success) 8%, var(--color-paper)); color: var(--color-success); font-size: .8rem; font-weight: 700; }
.completed-status svg { width: 1rem; fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 2; }
.progress-toggle:focus-visible,.failure-summary:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
.progress-toggle svg,.failure-summary svg { width: 1rem; fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.8; transition: transform .18s ease; }
.progress-toggle svg.expanded,.failure-summary svg.expanded { transform: rotate(180deg); }
.progress-detail { grid-column: 1 / -1; grid-row: 3; display: grid; gap: .75rem; border-top: 1px solid var(--color-border); padding-top: .75rem; }
.progress-detail.after-failure { grid-row: 4; }
.progress-detail header { display: flex; align-items: center; justify-content: space-between; gap: .75rem; }
.progress-detail header div { min-width: 0; display: grid; gap: .15rem; }
.progress-detail header strong { font-family: var(--font-literary); }
.progress-detail header span { color: var(--color-ink-muted); font-size: .78rem; }
.progress-detail header :deep(.app-button) { min-height: 2.5rem; flex: 0 0 auto; padding-block: .45rem; }
.failure { min-width: 0; margin: 0; overflow-wrap: anywhere; color: var(--color-danger); font-size: .78rem; line-height: 1.35; }
.compact-error,.notice { grid-column: 1 / -1; grid-row: 3; margin: 0; font-size: .78rem; }.compact-error.after-failure { grid-row: 4; }.notice { color: var(--color-ink-muted); }
@media (prefers-reduced-motion: reduce) { .progress-toggle svg,.failure-summary svg { transition: none; } }
@media (max-width: 35rem) { .candidate-action > :deep(.app-button),.progress-toggle,.completed-status { grid-column: 1; grid-row: 3; }.failure-summary { grid-row: 4; }.progress-detail,.compact-error,.notice { grid-row: 4; }.progress-detail.after-failure,.compact-error.after-failure { grid-row: 5; }.progress-detail header { align-items: stretch; flex-direction: column; }.progress-detail header :deep(.app-button) { width: 100%; } }
</style>
