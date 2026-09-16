<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import ImportPreferenceControl from './ImportPreferenceControl.vue';
import AppButton from '../../ui/components/AppButton.vue';
import { useImportQueue, type ImportTransfer } from './import-queue';
import { analysisErrorKey, importedTitle, importErrorKey } from './import-feedback';
export default defineComponent({
  components: { RouterLink, AppButton, ImportPreferenceControl },
  emits: ['review'],
  data: () => ({ queue: useImportQueue(), offset: 0 }),
  computed: {
    visible() { return this.queue.entries.slice(this.offset, this.offset + 25); },
    added(): number { return this.queue.entries.filter(item => item.state === 'added').length; },
    progress(): string {
      return [this.added ? this.$t('imports.flow.readyCount', { count: this.added }) : '', this.queue.outstanding ? this.$t('imports.flow.pendingCount', { count: this.queue.outstanding }) : '', this.queue.attention ? this.$t('imports.flow.checkCount', { count: this.queue.attention }) : ''].filter(Boolean).join(' · ');
    },
    canPause(): boolean { return this.queue.entries.some(item => item.state === 'queued'); },
  },
  watch: { 'queue.entries.length'() { this.offset = Math.min(this.offset, Math.floor(Math.max(0, this.queue.entries.length - 1) / 25) * 25); } },
  methods: {
    importedTitle, importErrorKey, analysisErrorKey,
    choose(event: Event) {
      const input = event.target as HTMLInputElement;
      this.queue.enqueue(Array.from(input.files || [])); input.value = '';
    },
    status(item: ImportTransfer): string {
      if (item.state === 'attention') return 'imports.flow.check';
      if (['acquired', 'adding'].includes(item.state)) return 'imports.flow.preparing';
      return `imports.flow.${item.state}`;
    },
  },
});
</script>

<template>
  <section class="import-upload" :aria-label="$t('imports.flow.title')">
    <div class="import-upload-bar">
      <div class="import-upload-copy"><h2>{{ $t('imports.flow.title') }}</h2><p>{{ $t('imports.flow.hint') }}</p></div>
      <label class="app-button app-button--primary import-picker">{{ $t('imports.flow.choose') }}<input type="file" accept=".txt,text/plain" multiple :aria-label="$t('imports.flow.choose')" @change="choose"></label>
    </div>
    <ImportPreferenceControl />
    <template v-if="queue.entries.length">
      <div class="import-progress-summary">
        <p role="status" aria-live="polite">{{ progress }}</p>
        <div class="app-actions import-actions">
          <AppButton v-if="queue.paused" variant="quiet" @click="queue.resume()">{{ $t('imports.resume') }}</AppButton>
          <AppButton v-else-if="canPause" variant="quiet" @click="queue.pause()">{{ $t('imports.flow.pauseUploads') }}</AppButton>
          <AppButton v-if="added > 0" variant="quiet" @click="queue.clearFinished(); offset = 0">{{ $t('imports.flow.clear') }}</AppButton>
        </div>
      </div>
      <p v-if="queue.paused" class="import-note">{{ $t('imports.flow.paused') }}</p>
      <ul class="import-list import-progress-list">
        <li v-for="item in visible" :key="item.key" :class="{ 'import-needs-attention': item.state === 'attention' }">
          <span class="import-state-mark" :class="{ 'is-complete': item.state === 'added', 'is-working': ['waiting', 'transferring', 'acquired', 'adding'].includes(item.state) }" aria-hidden="true">
            <svg v-if="item.state === 'added'" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2"><path d="m4 10 4 4 8-8" /></svg>
            <svg v-else-if="item.state === 'attention'" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M10 5v6m0 2v2" /></svg>
          </span>
          <div class="import-copy">
            <strong>{{ importedTitle(item.name) }}</strong>
            <span>{{ $t(status(item)) }}</span>
            <p v-if="item.error" class="import-error">{{ $t(importErrorKey(item.error)) }}</p>
            <p v-else-if="item.receipt?.state === 'analysis_failed'" class="import-error">{{ $t(analysisErrorKey(item.receipt.errorCode)) }}</p>
            <p v-if="item.warnings?.length" class="import-note">{{ $t('imports.flow.cleanupNote') }}</p>
          </div>
          <RouterLink v-if="item.libraryId" class="app-button app-button--secondary import-read" :to="{ name: 'reader', params: { bookId: item.libraryId } }">{{ $t('imports.flow.read') }}</RouterLink>
          <AppButton v-else-if="['review', 'attention'].includes(item.state) && item.receiptId" variant="secondary" @click="$emit('review', item.receiptId)">{{ $t('imports.flow.checkBook') }}</AppButton>
          <AppButton v-else-if="item.state === 'attention'" variant="quiet" @click="queue.remove(item.key)">{{ $t('imports.dismiss') }}</AppButton>
        </li>
      </ul>
      <div v-if="queue.entries.length > 25" class="app-actions import-actions">
        <AppButton variant="secondary" :disabled="offset === 0" @click="offset = Math.max(0, offset - 25)">{{ $t('imports.previous') }}</AppButton>
        <AppButton variant="secondary" :disabled="offset + 25 >= queue.entries.length" @click="offset += 25">{{ $t('imports.next') }}</AppButton>
      </div>
    </template>
  </section>
</template>
