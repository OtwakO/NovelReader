<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { CandidateOperationAttempt, CandidateOperationSnapshot } from './candidate-operation';

export default defineComponent({
  name: 'CandidateProgressDetails',
  props: {
    snapshot: { type: Object as PropType<CandidateOperationSnapshot>, required: true },
    failureLimit: { type: Number, default: 3 },
  },
  computed: {
    attempts(): CandidateOperationAttempt[] { return this.snapshot.attempts ?? []; },
    verifiedAttempt(): CandidateOperationAttempt | undefined { return this.attempts.find((attempt) => attempt.state === 'verified'); },
    activeAttempts(): CandidateOperationAttempt[] { return this.attempts.filter((attempt) => attempt.state === 'running'); },
    readyAttempts(): CandidateOperationAttempt[] { return this.attempts.filter((attempt) => attempt.state === 'ready'); },
    failedAttempts(): CandidateOperationAttempt[] { return this.attempts.filter((attempt) => attempt.state === 'failed'); },
    recentFailures(): CandidateOperationAttempt[] { return this.failedAttempts.slice(-this.failureLimit); },
    queuedCount(): number { return this.attempts.filter((attempt) => attempt.state === 'queued').length; },
    skippedCount(): number { return this.attempts.filter((attempt) => attempt.state === 'skipped').length; },
    hiddenFailureCount(): number { return Math.max(0, this.failedAttempts.length - this.recentFailures.length); },
    winnerFound(): boolean { return Boolean(this.verifiedAttempt); },
  },
  methods: {
    attemptName(attempt: CandidateOperationAttempt): string { return attempt.sourceName || attempt.sourceUrl; },
    attemptStatus(attempt: CandidateOperationAttempt): string {
      if (attempt.state === 'failed') return this.$t('candidate.state.failed');
      if (attempt.state === 'verified') return this.$t('candidate.state.verified');
      if (attempt.state === 'ready') return this.$t('candidate.state.ready');
      if (attempt.state === 'skipped') return this.$t('candidate.state.skipped');
      if (attempt.stage) return this.$t(`candidate.stage.${attempt.stage}`);
      return this.$t('candidate.state.running');
    },
  },
});
</script>

<template>
  <div class="candidate-progress-details">
    <p class="counts">
      <template v-if="winnerFound">
        {{ $t('candidate.winnerCounts', { active: snapshot.active, skipped: skippedCount }) }}
      </template>
      <template v-else>
        {{ $t('candidate.counts', { completed: snapshot.completed, known: snapshot.known, active: snapshot.active }) }}
      </template>
    </p>

    <ul v-if="verifiedAttempt || readyAttempts.length || activeAttempts.length || recentFailures.length" class="attempt-list">
      <li v-if="verifiedAttempt" class="verified">
        <span class="state-mark" aria-hidden="true" />
        <strong>{{ attemptName(verifiedAttempt) }}</strong>
        <span>{{ attemptStatus(verifiedAttempt) }}</span>
      </li>
      <li v-for="attempt in readyAttempts" :key="`${attempt.sourceUrl}-ready`" class="ready">
        <span class="state-mark" aria-hidden="true" />
        <strong>{{ attemptName(attempt) }}</strong>
        <span>{{ attemptStatus(attempt) }}</span>
      </li>
      <li v-for="attempt in activeAttempts" :key="`${attempt.sourceUrl}-active`" class="active">
        <span class="state-mark" aria-hidden="true" />
        <strong>{{ attemptName(attempt) }}</strong>
        <span>{{ attemptStatus(attempt) }}</span>
      </li>
      <li v-for="attempt in recentFailures" :key="`${attempt.sourceUrl}-failed`" class="failed">
        <span class="state-mark" aria-hidden="true" />
        <strong>{{ attemptName(attempt) }}</strong>
        <span>{{ attemptStatus(attempt) }}</span>
      </li>
    </ul>

    <div v-if="queuedCount || skippedCount || hiddenFailureCount" class="summaries">
      <span v-if="queuedCount">{{ $t('candidate.queued', { count: queuedCount }) }}</span>
      <span v-if="skippedCount">{{ $t('candidate.skipped', { count: skippedCount }) }}</span>
      <span v-if="hiddenFailureCount">{{ $t('candidate.moreFailed', { count: hiddenFailureCount }) }}</span>
    </div>
  </div>
</template>

<style scoped>
.candidate-progress-details { display: grid; gap: .6rem; }
.counts { margin: 0; color: var(--color-ink-muted); font-size: .78rem; }
.attempt-list { display: grid; gap: .4rem; margin: 0; padding: 0; list-style: none; }
.attempt-list li { min-width: 0; display: grid; grid-template-columns: .65rem minmax(0, 1fr) auto; align-items: center; gap: .55rem; color: var(--color-ink-muted); font-size: .8rem; }
.attempt-list strong { min-width: 0; overflow: hidden; color: var(--color-ink); text-overflow: ellipsis; white-space: nowrap; }
.state-mark { width: .55rem; height: .55rem; border: 2px solid currentColor; border-radius: 50%; }
.active { color: var(--color-accent); }.active .state-mark { border-top-color: transparent; animation: spin .8s linear infinite; }
.failed { color: var(--color-danger); }.failed .state-mark,.verified .state-mark,.ready .state-mark { background: currentColor; }
.verified { color: var(--color-success); }.ready { color: var(--color-accent-strong); }
.summaries { display: flex; flex-wrap: wrap; gap: .4rem 1rem; color: var(--color-ink-muted); font-size: .78rem; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .active .state-mark { animation: none; } }
@media (max-width: 35rem) {
  .attempt-list li { grid-template-columns: .65rem minmax(0, 1fr); }
  .attempt-list li > :last-child { grid-column: 2; }
}
</style>
