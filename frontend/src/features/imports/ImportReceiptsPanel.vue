<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import AppButton from '../../ui/components/AppButton.vue';
import { listImportHistory, historyStatuses, type ImportHistoryItem, type HistoryStatus } from '../../api/import-history';
import type { ImportFormat } from '../../api/file-imports';
import { ApiError } from '../../api/transport';
import { acceptImport, additionDetails, receiptFormat, receiptState, type ImportReceipt } from './import-format';
import { useImportQueue } from './import-queue';
import { encoderNoticeKey, importErrorKey } from './import-feedback';
import { createImportTask } from './import-task';
export default defineComponent({
  components: { RouterLink, AppButton },
  emits: ['review'],
  data: () => ({
    queue: useImportQueue(), task: createImportTask(), records: [] as ImportHistoryItem[], format: '' as ImportFormat | '', notices: {} as Record<string, string[]>, next: '',
    selected: [] as ImportReceipt[], after: '', filter: '' as HistoryStatus | '', added: undefined as number | undefined,
    failures: [] as { id: string; name: string; cause: unknown }[], timer: undefined as ReturnType<typeof setInterval> | undefined,
    states: historyStatuses,
  }),
  computed: {
    listingKey(): string { return `${this.format}:${this.filter}:${this.after}`; },
  },
  watch: { listingKey() { this.changePage(); }, 'queue.revision'() { this.refresh(); } },
  mounted() {
    this.refresh();
    this.timer = setInterval(() => {
      if (document.visibilityState === 'visible' && (this.queue.running || this.records.some(item => item.status === 'processing'))) this.refresh();
    }, 5000);
  },
  beforeUnmount() { clearInterval(this.timer); this.task.cancel(); },
  methods: {
    importErrorKey, encoderNoticeKey, receiptFormat,
    statusKey(item: ImportReceipt): string { const state = receiptState(item); return receiptFormat(item) === 'epub' && ['received', 'analyzing', 'analysis_failed'].includes(state) ? `imports.epub.${state === 'analysis_failed' ? 'preparationFailed' : 'preparing'}` : `imports.state.${state}`; },
    async load(signal: AbortSignal) {
      // Completion can arrive while the task is busy; don't lose that refresh.
      let revision: number;
      do {
        revision = this.queue.revision;
        const page = await listImportHistory(this.format, this.filter, this.after, signal);
        signal.throwIfAborted(); this.records = page.items; this.next = page.nextCursor || '';
      } while (revision !== this.queue.revision);
    },
    refresh() { void this.task.run(this.load); },
    changePage() {
      this.task.cancel();
      this.records = []; this.next = ''; this.selected = []; this.notices = {};
      this.refresh();
    },
    page(cursor: string) { this.after = cursor; },
    setFormat(format: ImportFormat | '') { this.after = ''; this.format = format; },
    setFilter(status: HistoryStatus | '') { this.after = ''; this.filter = status; },
    toggle(item: ImportReceipt, checked: boolean) {
      this.selected = this.selected.filter(value => value.id !== item.id);
      if (checked) this.selected.push({ ...item }); // Retain the selected version, never silently approve a newer one.
    },
    acceptSelection() {
      const batch = [...this.selected];
      void this.task.run(async signal => {
        this.added = 0; this.failures = [];
        for (const item of batch) {
          try {
            const details = await additionDetails(item, signal);
            signal.throwIfAborted(); this.notices[item.id] = details.notices;
            if (details.needsReview) throw new ApiError(409, { code: 'epub_review_required' });
            await acceptImport(item, details, signal);
            signal.throwIfAborted(); this.added++;
            this.queue.libraryRevision++;
            this.selected = this.selected.filter(value => value.id !== item.id);
          } catch (cause) { signal.throwIfAborted(); this.failures.push({ id: item.id, name: item.originalName, cause }); }
        }
        await this.load(signal);
      });
    },
  },
});
</script>

<template>
  <section class="import-section" aria-labelledby="receipts-title">
    <h2 id="receipts-title">{{ $t('imports.results') }}</h2>
    <p>{{ $t('imports.resultsHint') }}</p>
    <div class="app-actions import-toolbar">
      <label>{{ $t('imports.format') }} <select :value="format" :disabled="task.busy" @change="setFormat(($event.target as HTMLSelectElement).value as ImportFormat | '')"><component :is="'button'" type="button"><selectedcontent /></component><option value="">{{ $t('imports.all') }}</option><option value="txt">TXT</option><option value="epub">EPUB</option></select></label>
      <label>{{ $t('imports.filter') }} <select :value="filter" :disabled="task.busy" @change="setFilter(($event.target as HTMLSelectElement).value as HistoryStatus | '')"><component :is="'button'" type="button"><selectedcontent /></component><option value="">{{ $t('imports.all') }}</option><option v-for="state in states" :key="state" :value="state">{{ $t(`imports.historyStatus.${state}`) }}</option></select></label>
      <AppButton variant="secondary" :busy="task.busy" @click="refresh">{{ $t('imports.refresh') }}</AppButton>
      <AppButton :disabled="!selected.length || task.busy" @click="acceptSelection">{{ $t('imports.addSelected', { count: selected.length }) }}</AppButton>
    </div>
    <p v-if="task.error" role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
    <p v-if="added !== undefined" role="status">{{ $t('imports.batchResult', { added, failed: failures.length }) }}</p>
    <p v-for="failure in failures" :key="failure.id" class="import-error">{{ failure.name }}: {{ $t(importErrorKey(failure.cause)) }}</p>
    <p v-if="!records.length && !task.busy">{{ $t('imports.noResults') }}</p>
    <ul class="import-list">
      <li v-for="{ receipt: item, status, format: itemFormat } in records" :key="`${itemFormat}:${item.id}`">
        <label class="import-selection"><input type="checkbox" :aria-label="$t('imports.selectFile', { name: item.originalName })" :checked="selected.some(value => value.id === item.id)" :disabled="status !== 'ready' || task.busy" @change="toggle(item, ($event.target as HTMLInputElement).checked)"></label>
        <div class="import-copy"><strong>{{ item.originalName }}</strong><span>{{ itemFormat.toUpperCase() }} · {{ $t(status === 'needs_review' ? 'imports.historyStatus.needs_review' : statusKey(item)) }}</span><p v-for="notice in notices[item.id] || []" :key="notice" class="import-note">{{ $t(encoderNoticeKey(notice)) }}</p></div>
        <RouterLink v-if="item.libraryId && item.state !== 'removing'" class="app-button app-button--secondary import-read" :to="{ name: 'book-detail', params: { bookId: item.libraryId } }">{{ $t('bookDetail.title') }}</RouterLink>
        <AppButton v-else variant="secondary" @click="$emit('review', item.id, receiptFormat(item))">{{ $t('imports.flow.checkBook') }}</AppButton>
      </li>
    </ul>
    <div v-if="after || next" class="app-actions import-actions">
      <AppButton variant="secondary" :disabled="!after || task.busy" @click="page('')">{{ $t('imports.first') }}</AppButton>
      <AppButton variant="secondary" :disabled="!next || task.busy" @click="page(next)">{{ $t('imports.next') }}</AppButton>
    </div>
  </section>
</template>
