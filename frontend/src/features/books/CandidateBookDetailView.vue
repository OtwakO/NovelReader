<script lang="ts">
import { defineComponent } from 'vue';
import type { SearchResult } from '../../api/search';
import AppButton from '../../ui/components/AppButton.vue';
import FeatureScaffold from '../../ui/components/FeatureScaffold.vue';
import WebViewFailureHint from '../../ui/components/WebViewFailureHint.vue';
import CandidateProgressDetails from '../candidates/CandidateProgressDetails.vue';
import {
  cancelCandidateOperation,
  candidateOperationMatches,
  commitCandidateOperation,
  forgetCandidateOperation,
  getCandidateOperation,
  recalledCandidateOperation,
  rememberCandidateCommitted,
  rememberCandidateOperation,
  startCandidateOperation,
  subscribeCandidateOperation,
  type CandidateOperationSnapshot,
} from '../candidates/candidate-operation';
import { loadCandidateSelection } from '../search/candidate-selection';
import BookCover from './BookCover.vue';
import BookDetailSection from './BookDetailSection.vue';
import { readableChapterLabel } from './book-display';

export default defineComponent({
  name: 'CandidateBookDetailView',
    components: { AppButton, BookCover, BookDetailSection, CandidateProgressDetails, FeatureScaffold, WebViewFailureHint },
  data() {
    return {
      candidate: null as SearchResult | null,
      operation: null as CandidateOperationSnapshot | null,
      stream: null as EventSource | null,
      loading: true,
      cancelling: false,
      shelving: false,
      error: '',
      shelfError: '',
    };
  },
  computed: {
    preview() { return this.operation?.preview || null; },
    backRoute(): string { return this.$route.query.from === 'explore' ? '/explore' : '/search'; },
    backLabel(): string { return this.$route.query.from === 'explore' ? this.$t('candidateBookDetail.backExplore') : this.$t('candidateBookDetail.back'); },
    sourceName(): string { return this.preview?.selection.selectedSourceName || this.candidate?.sourceName || ''; },
    displayLastChapter(): string {
      return readableChapterLabel(this.preview?.book.lastChapter);
    },
    running(): boolean { return this.operation?.state === 'running'; },
    automaticCommitPending(): boolean { return this.operation?.state === 'verified' && Boolean(this.operation.automaticCommit); },
    terminalFailure(): boolean {
      return Boolean(!this.preview && this.operation && (['exhausted', 'cancelled'].includes(this.operation.state) || (this.operation.state === 'failed' && !this.operation.commitPending)));
    },
  },
  async mounted() { await this.load(); },
  beforeUnmount() { this.stream?.close(); },
  methods: {
    async load() {
      const key = typeof this.$route.query.candidate === 'string' ? this.$route.query.candidate : '';
      this.candidate = loadCandidateSelection(key);
      if (!this.candidate) { this.loading = false; this.error = this.$t('candidateBookDetail.missing'); return; }
      if (this.candidate.shelfBookId) {
        void this.$router.replace(`/books/${encodeURIComponent(this.candidate.shelfBookId)}`);
        return;
      }
      this.loading = true;
      this.error = '';
      const remembered = recalledCandidateOperation(this.candidate);
      if (remembered) {
        try {
          const snapshot = await getCandidateOperation(remembered);
          if (!candidateOperationMatches(this.candidate, snapshot)) {
            if (this.followsUpdates(snapshot)) await cancelCandidateOperation(remembered).catch(() => undefined);
            forgetCandidateOperation(this.candidate);
          } else {
            this.accept(snapshot);
            if (this.followsUpdates(snapshot)) this.connect(snapshot.id);
            this.loading = false;
            return;
          }
        } catch {
          forgetCandidateOperation(this.candidate);
        }
      }
      await this.start();
    },
    async start() {
      if (!this.candidate || this.running) return;
      this.stream?.close();
      forgetCandidateOperation(this.candidate);
      this.operation = null;
      this.loading = true;
      this.cancelling = false;
      this.error = '';
      this.shelfError = '';
      try {
        const snapshot = await startCandidateOperation(this.candidate);
        rememberCandidateOperation(this.candidate, snapshot.id);
        this.accept(snapshot);
        if (this.followsUpdates(snapshot)) this.connect(snapshot.id);
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t('candidateBookDetail.failed');
      } finally {
        this.loading = false;
      }
    },
    connect(id: string) {
      this.stream?.close();
      this.stream = subscribeCandidateOperation(id, {
        onSnapshot: (snapshot) => this.accept(snapshot),
        onDisconnect: () => { this.error = this.$t('candidateBookDetail.disconnected'); },
      });
    },
    accept(snapshot: CandidateOperationSnapshot) {
      this.operation = snapshot;
      this.error = '';
      this.cancelling = false;
      if (!this.followsUpdates(snapshot)) this.stream?.close();
      if (snapshot.state === 'committed' && snapshot.storedBook && this.candidate) {
        rememberCandidateCommitted(this.candidate, snapshot.storedBook.id);
        void this.$router.replace(`/books/${encodeURIComponent(snapshot.storedBook.id)}`);
      }
    },
    async cancel() {
      if (!this.operation || this.cancelling) return;
      this.cancelling = true;
      this.error = '';
      try {
        await cancelCandidateOperation(this.operation.id);
      } catch (cause) {
        this.cancelling = false;
        this.error = cause instanceof Error ? cause.message : this.$t('candidate.cancelFailed');
      }
    },
    async addToShelf() {
      if (!this.operation?.preview || this.shelving || this.automaticCommitPending) return;
      this.shelving = true;
      this.shelfError = '';
      try {
        const id = this.operation.commitPending ? '' : (crypto.randomUUID?.() ?? `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`);
        this.accept(await commitCandidateOperation(this.operation.id, id));
      } catch (cause) {
        this.shelfError = cause instanceof Error ? cause.message : this.$t('candidateBookDetail.shelfFailed');
      } finally {
        this.shelving = false;
      }
    },
    followsUpdates(snapshot: CandidateOperationSnapshot) {
      return snapshot.state === 'running' || (snapshot.state === 'verified' && Boolean(snapshot.automaticCommit));
    },
  },
});
</script>

<template>
  <FeatureScaffold :title="preview?.book.name || candidate?.name || $t('candidateBookDetail.title')" :description="$t('candidateBookDetail.description')">
    <section v-if="!preview && !terminalFailure" class="state" aria-live="polite">
      <div class="state-heading">
        <strong>{{ $t('candidate.title') }}</strong>
        <span>{{ $t('candidate.help') }}</span>
      </div>
      <p v-if="loading && !operation" aria-busy="true">{{ $t('candidate.starting') }}</p>
      <CandidateProgressDetails v-if="operation" :snapshot="operation" :failure-limit="5" />
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <AppButton v-if="operation" variant="danger" :busy="cancelling" @click="cancel">
        {{ cancelling ? $t('candidate.cancelling') : $t('candidateBookDetail.cancel') }}
      </AppButton>
    </section>

    <section v-else-if="terminalFailure || (error && !preview)" class="state">
      <p role="alert">{{ error || operation?.message || $t('candidateBookDetail.failed') }}</p>
      <WebViewFailureHint />
      <div class="state-actions">
        <AppButton variant="secondary" @click="start">{{ $t('candidateBookDetail.retry') }}</AppButton>
        <RouterLink class="secondary-link" :to="backRoute">{{ backLabel }}</RouterLink>
      </div>
    </section>

    <template v-else-if="candidate && preview">
      <section class="hero">
        <BookCover class="cover" :name="preview.book.name" :url="preview.book.coverDisplayUrl || ''" :alt="$t('bookDetail.coverAlt', { name: preview.book.name })" />
        <div class="copy">
          <p class="author">{{ preview.book.author || $t('app.common.unknownAuthor') }}</p>
          <p v-if="preview.book.kind || displayLastChapter" class="meta">
            <span v-if="preview.book.kind">{{ preview.book.kind }}</span>
            <span v-if="displayLastChapter">{{ displayLastChapter }}</span>
          </p>
          <p class="source">{{ $t('candidateBookDetail.source', { source: sourceName }) }} · {{ $t('candidateBookDetail.sourceCount', { count: operation?.known || 1 }) }}</p>
          <p v-if="preview.selection.usedFallback" class="fallback" role="status">{{ $t('candidateBookDetail.fallback', { source: sourceName }) }}</p>
          <div class="hero-actions">
            <AppButton :busy="shelving || automaticCommitPending" @click="addToShelf">{{ shelving || automaticCommitPending ? $t('candidateBookDetail.shelving') : $t('candidateBookDetail.shelve') }}</AppButton>
            <RouterLink class="secondary-link" :to="backRoute">{{ backLabel }}</RouterLink>
          </div>
        </div>
      </section>

      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <BookDetailSection :title="$t('bookDetail.synopsis')">
        <template #body>
          <p class="intro">{{ preview.book.intro || $t('candidateBookDetail.noIntro') }}</p>
        </template>
      </BookDetailSection>
      <p v-if="shelfError" class="error" role="alert">{{ shelfError }}</p>
      <div class="bottom-actions">
        <AppButton :busy="shelving || automaticCommitPending" @click="addToShelf">{{ shelving || automaticCommitPending ? $t('candidateBookDetail.shelving') : $t('candidateBookDetail.shelve') }}</AppButton>
        <RouterLink class="secondary-link" :to="backRoute">{{ backLabel }}</RouterLink>
      </div>
    </template>
  </FeatureScaffold>
</template>

<style scoped>
.state { display: grid; gap: 1rem; padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.state-heading { display: grid; gap: .2rem; }.state-heading strong { font-family: var(--font-literary); }.state-heading span { color: var(--color-ink-muted); font-size: .82rem; }
.state-actions,.hero-actions,.bottom-actions { display: flex; flex-wrap: wrap; align-items: center; gap: .75rem; }.bottom-actions { margin-top: 1rem; }
.hero { display: grid; grid-template-columns: 9rem minmax(0,1fr); gap: 1.5rem; align-items: start; padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.cover { width: 9rem; aspect-ratio: 2/3; object-fit: contain; background: var(--color-paper-muted); }
.copy { min-width: 0; }.author,.meta { color: var(--color-ink-muted); }.author { margin-top: 0; }.meta { display: flex; flex-wrap: wrap; gap: .35rem .75rem; }
.source { color: var(--color-accent-strong); overflow-wrap: anywhere; }.fallback { max-width: 44rem; border: 1px solid color-mix(in srgb,var(--color-accent) 32%,var(--color-border)); border-radius: var(--radius-md); padding: .55rem .7rem; background: var(--color-accent-soft); color: var(--color-accent-strong); font-size: .82rem; line-height: 1.45; overflow-wrap: anywhere; }
.intro { max-width: 72ch; line-height: 1.75; white-space: pre-line; overflow-wrap: anywhere; }
.secondary-link { min-height: 2.75rem; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .65rem 1rem; background: var(--color-paper-raised); color: var(--color-ink); text-decoration: none; font-weight: 700; }
.secondary-link:hover { border-color: color-mix(in srgb,var(--color-accent) 55%,var(--color-border)); background: var(--color-paper); }.secondary-link:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
.error { margin: 0; color: var(--color-danger); overflow-wrap: anywhere; }
@media (max-width: 38rem) {
  .hero { grid-template-columns: 6rem minmax(0,1fr); gap: 1rem; }.cover { width: 6rem; }.hero-actions { grid-column: 1/-1; }.hero-actions :deep(.app-button),.hero-actions .secondary-link,.bottom-actions :deep(.app-button),.bottom-actions .secondary-link { width: 100%; }
}
</style>
