<script lang="ts">
import { defineComponent } from 'vue';
import AppButton from '../../../ui/components/AppButton.vue';
import WebViewFailureHint from '../../../ui/components/WebViewFailureHint.vue';

export default defineComponent({
  name: 'SearchStatus', components: { AppButton, WebViewFailureHint },
  props: { checked: { type: Number, required: true }, eligible: { type: Number, required: true }, resultCount: { type: Number, required: true }, resultLabelKey: { type: String, default: 'search.status.results' }, searching: Boolean, concurrency: { type: Number, required: true }, sourceFailures: { type: Number, required: true }, errorCode: { type: String, required: true }, errorDetail: { type: String, required: true }, storageWarning: Boolean, restartRequired: Boolean, retryRequired: Boolean, hasMore: Boolean, moreCount: { type: Number, required: true } },
  emits: ['restart', 'retry', 'more'],
  computed: { percent(): number { return this.eligible > 0 ? Math.min(100, Math.round((this.checked / this.eligible) * 100)) : 0; } },
});
</script>

<template>
  <section class="status" aria-live="polite">
    <div class="progress" role="progressbar" :aria-valuenow="checked" aria-valuemin="0" :aria-valuemax="eligible || undefined"><span :style="{ transform: `scaleX(${percent / 100})` }" /></div>
    <div class="status-row"><strong>{{ eligible ? $t('search.status.checkedOf', { checked, eligible }) : $t('search.status.checked', { checked }) }}</strong><span>{{ $t(resultLabelKey, { count: resultCount }) }}</span><span v-if="searching">{{ $t('search.status.concurrency', { count: concurrency }) }}</span></div>
    <p v-if="sourceFailures" class="hint">{{ $t('search.status.failures', { count: sourceFailures }) }}</p>
    <WebViewFailureHint v-if="sourceFailures" />
    <p v-if="errorCode === 'disconnect'" class="error" role="alert">{{ $t('search.status.disconnected') }}</p>
    <p v-else-if="errorCode === 'stale'" class="error" role="alert">{{ $t('search.status.stale') }}<span v-if="errorDetail"> {{ errorDetail }}</span></p>
    <p v-if="storageWarning" class="error" role="alert">{{ $t('search.status.storage') }}</p>
    <div v-if="!searching" class="actions"><AppButton v-if="restartRequired" @click="$emit('restart')">{{ $t('search.actions.restart') }}</AppButton><AppButton v-else-if="retryRequired" @click="$emit('retry')">{{ $t('search.actions.retry') }}</AppButton><AppButton v-else-if="hasMore" @click="$emit('more')">{{ $t('search.actions.more', { count: moreCount }) }}</AppButton></div>
  </section>
</template>

<style scoped>
.status { padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.progress { height: .45rem; overflow: hidden; border-radius: 999px; background: var(--color-paper-muted); }
.progress span { display: block; width: 100%; height: 100%; transform-origin: left; background: var(--color-accent); transition: transform 160ms ease-out; }
.status-row { display: flex; flex-wrap: wrap; justify-content: space-between; gap: .4rem 1rem; margin-top: .75rem; color: var(--color-ink-muted); font-size: .82rem; }
.status-row strong { color: var(--color-ink); }.hint, .error { margin: .65rem 0 0; font-size: .84rem; }.hint { color: var(--color-warm); }.error { color: var(--color-danger); }.actions { margin-top: .8rem; }
</style>
