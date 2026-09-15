<script lang="ts">
import AppDisclosure from '../../ui/components/AppDisclosure.vue';
import { defineComponent, nextTick } from 'vue';
import ImportQueuePanel from './ImportQueuePanel.vue';
import ImportReceiptsPanel from './ImportReceiptsPanel.vue';
import ImportInboxPanel from './ImportInboxPanel.vue';
import ImportReviewView from './ImportReviewView.vue';
import type { TXTReceipt } from '../../api/txt-imports';
import { useImportQueue } from './import-queue';
import './imports.css';
export default defineComponent({
  components: { AppDisclosure, ImportQueuePanel, ImportReceiptsPanel, ImportInboxPanel, ImportReviewView },
  props: { initialReview: { type: String, default: '' } },
  emits: ['review-closed'],
  data: () => ({ queue: useImportQueue(), selected: '', historyOpen: false, inboxOpen: false, returnFocus: undefined as HTMLElement | undefined }),
  watch: { initialReview: { immediate: true, handler(value: string) { if (value) void this.review(value); } } },
  methods: {
    async review(id: string) {
      this.returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : undefined;
      this.selected = id;
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
    updated(receipt: TXTReceipt) {
      const inQueue = this.queue.entries.some(item => item.receiptId === receipt.id);
      this.queue.updateReceipt(receipt);
      if (inQueue && receipt.state === 'published') void this.closeReview();
    },
    removed(id: string) {
      const entry = this.queue.entries.find(item => item.receiptId === id);
      if (entry) this.queue.remove(entry.key);
      (this.$refs.history as InstanceType<typeof ImportReceiptsPanel> | undefined)?.refresh();
    },
  },
});
</script>
<template>
  <div class="imports-page import-workspace">
    <ImportQueuePanel @review="review" />
    <ImportReviewView v-if="selected" :key="selected" :receipt-id="selected" @updated="updated" @removed="removed" @close="closeReview" />
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
