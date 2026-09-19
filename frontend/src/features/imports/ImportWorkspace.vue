<script lang="ts">
import AppDisclosure from '../../ui/components/AppDisclosure.vue';
import { defineComponent, nextTick, type PropType } from 'vue';
import ImportQueuePanel from './ImportQueuePanel.vue';
import ImportReceiptsPanel from './ImportReceiptsPanel.vue';
import ImportInboxPanel from './ImportInboxPanel.vue';
import ImportReviewView from './ImportReviewView.vue';
import EPUBReviewView from './EPUBReviewView.vue';
import type { ImportFormat } from '../../api/file-imports';
import { receiptFormat, receiptState, type ImportReceipt } from './import-format';
import { useImportQueue } from './import-queue';
import './imports.css';
export default defineComponent({
  components: { AppDisclosure, ImportQueuePanel, ImportReceiptsPanel, ImportInboxPanel, ImportReviewView, EPUBReviewView },
  props: { initialReview: { type: String, default: '' }, initialFormat: { type: String as PropType<ImportFormat>, default: 'txt' } },
  emits: ['review-closed'],
  data: () => ({ queue: useImportQueue(), selected: '', format: 'txt' as ImportFormat, historyOpen: false, inboxOpen: false, returnFocus: undefined as HTMLElement | undefined }),
  computed: { initialSelection(): string { return `${this.initialFormat}:${this.initialReview}`; } },
  watch: { initialSelection: { immediate: true, handler() { if (this.initialReview) void this.review(this.initialReview, this.initialFormat); } } },
  methods: {
    async review(id: string, format: ImportFormat = 'txt') {
      this.returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : undefined;
      this.selected = id; this.format = format;
      await nextTick();
      const heading = (this.$el as HTMLElement).querySelector<HTMLElement>('#import-review-title');
      heading?.focus({ preventScroll: true });
      heading?.scrollIntoView?.({ block: 'start', behavior: 'auto' });
    },
    async closeReview() {
      this.selected = ''; this.$emit('review-closed');
      await nextTick();
      if (this.returnFocus?.isConnected) this.returnFocus.focus();
      else (this.$el as HTMLElement).querySelector<HTMLInputElement>('input[type="file"]')?.focus();
    },
    updated(receipt: ImportReceipt) {
      const inQueue = this.queue.entries.some(item => item.receiptId === receipt.id && item.format === receiptFormat(receipt));
      this.queue.updateReceipt(receipt);
      if (inQueue && receiptState(receipt) === 'published') void this.closeReview();
    },
    removed(id: string) {
      const entry = this.queue.entries.find(item => item.receiptId === id && item.format === this.format);
      if (entry) this.queue.remove(entry.key);
      (this.$refs.history as InstanceType<typeof ImportReceiptsPanel> | undefined)?.refresh();
    },
  },
});
</script>
<template>
  <div class="imports-page import-workspace">
    <ImportQueuePanel @review="review" />
    <component :is="format === 'epub' ? 'EPUBReviewView' : 'ImportReviewView'" v-if="selected" :key="`${format}:${selected}`" :receipt-id="selected" @updated="updated" @removed="removed" @close="closeReview" />
    <div class="import-secondary">
      <AppDisclosure v-model:open="historyOpen">
        <template #summary>{{ $t('imports.flow.history') }}</template>
        <ImportReceiptsPanel v-if="historyOpen" ref="history" @review="review" />
      </AppDisclosure>
      <AppDisclosure v-model:open="inboxOpen">
        <template #summary>{{ $t('imports.flow.serverFiles') }}</template>
        <ImportInboxPanel v-if="inboxOpen" />
      </AppDisclosure>
    </div>
  </div>
</template>
