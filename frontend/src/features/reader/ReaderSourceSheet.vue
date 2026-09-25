<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { Book } from '../../api/models';
import AppButton from '../../ui/components/AppButton.vue';
import SourceRecoveryPanel from '../source-recovery/SourceRecoveryPanel.vue';
export default defineComponent({
  name: 'ReaderSourceSheet',
  components: { AppButton, SourceRecoveryPanel },
  props: {
    book: { type: Object as PropType<Book | null>, default: null },
    currentSource: { type: String, required: true },
    loading: Boolean,
    metadataError: { type: String, default: '' },
    switching: Boolean,
    actionError: { type: String, default: '' },
    actionMessage: { type: String, default: '' },
    onClearAndRescan: { type: Function as PropType<() => Promise<void>>, required: true },
  },
  emits: ['close', 'matches', 'select', 'retry'],
});
</script>
<template>
  <div class="overlay" @click.self="$emit('close')" @keydown.esc="$emit('close')">
    <section class="sheet" role="dialog" aria-modal="true" :aria-label="$t('reader.sources.title')">
      <div class="sheet-scroll">
        <header>
          <div><h2>{{ $t('reader.sources.title') }}</h2><p v-if="book">{{ $t('reader.sources.current', { source: currentSource }) }}</p></div>
          <AppButton variant="quiet" @click="$emit('close')">{{ $t('reader.close') }}</AppButton>
        </header>
        <p v-if="loading" role="status" aria-busy="true">{{ $t('app.common.loading') }}</p>
        <div v-else-if="metadataError"><p role="alert">{{ metadataError }}</p><AppButton @click="$emit('retry')">{{ $t('app.common.retry') }}</AppButton></div>
        <SourceRecoveryPanel v-else-if="book" :book="book" :switching="switching" :action-error="actionError" :action-message="actionMessage" :on-clear-and-rescan="onClearAndRescan" @matches="$emit('matches', $event)" @select="$emit('select', $event)" />
      </div>
    </section>
  </div>
</template>
<style scoped>.overlay{position:fixed;z-index:90;inset:0;display:flex;align-items:flex-end;justify-content:center;background:rgb(0 0 0/.38)}.sheet{width:min(48rem,100%);max-height:88dvh;overflow:hidden;border-radius:var(--radius-lg) var(--radius-lg) 0 0;background:var(--color-paper-raised);color:var(--color-ink)}.sheet-scroll{max-height:88dvh;overflow:auto;overscroll-behavior:contain;padding:1rem}header{display:flex;justify-content:space-between;align-items:center;gap:1rem;margin-bottom:.8rem}h2,p{margin:0}h2{font:var(--weight-strong) var(--text-subheading) var(--font-literary)}p{margin-top:.2rem;color:var(--color-ink-muted);overflow-wrap:anywhere}:deep(.recovery){border:0;padding:0;background:transparent}</style>