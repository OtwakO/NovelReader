<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import AppButton from '../../ui/components/AppButton.vue';
import { acceptTXT, listTXTReceipts, type TXTReceipt } from '../../api/txt-imports';
import { useImportQueue } from './import-queue';
import { importedTitle, importErrorKey } from './import-feedback';
import { createImportTask } from './import-task';
export default defineComponent({
  components: { RouterLink, AppButton },
  emits: ['review'],
  data: () => ({
    queue: useImportQueue(), task: createImportTask(), records: [] as TXTReceipt[], next: '',
    selected: [] as TXTReceipt[], after: '', filter: '', added: undefined as number | undefined,
    failures: [] as { id: string; name: string; cause: unknown }[], timer: undefined as ReturnType<typeof setInterval> | undefined,
    states: ['needs_review', 'analysis_failed', 'ready', 'published'],
  }),
  computed: {
    visibleRecords(): TXTReceipt[] { return this.records.filter(item => !this.queue.entries.some(entry => entry.receiptId === item.id)); },
  },
  watch: { after() { this.changePage(); }, filter() { this.changePage(); }, 'queue.revision'() { this.refresh(); } },
  mounted() {
    this.refresh();
    this.timer = setInterval(() => {
      if (document.visibilityState === 'visible' && (this.queue.running || this.records.some(item => ['receiving', 'received', 'analyzing'].includes(item.state)))) this.refresh();
    }, 5000);
  },
  beforeUnmount() { clearInterval(this.timer); this.task.cancel(); },
  methods: {
    importErrorKey,
    async load(signal: AbortSignal) {
      const page = await listTXTReceipts(this.after, this.filter, signal);
      signal.throwIfAborted(); this.records = page.items; this.next = page.nextCursor || '';
    },
    refresh() { void this.task.run(this.load); },
    changePage() { this.task.cancel(); this.selected = []; this.refresh(); },
    page(cursor: string, state?: string) { this.after = cursor; if (state !== undefined) this.filter = state; },
    toggle(item: TXTReceipt, checked: boolean) {
      this.selected = this.selected.filter(value => value.id !== item.id);
      if (checked) this.selected.push({ ...item }); // Retain the selected version, never silently approve a newer one.
    },
    acceptSelection() {
      const batch = [...this.selected];
      void this.task.run(async signal => {
        this.added = 0; this.failures = [];
        for (const item of batch) {
          try {
            await acceptTXT(item.id, item.analysisVersion, importedTitle(item.originalName), '', signal);
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
      <label>{{ $t('imports.filter') }} <select :value="filter" :disabled="task.busy" @change="page('', ($event.target as HTMLSelectElement).value)"><component :is="'button'" type="button"><selectedcontent /></component><option value="">{{ $t('imports.all') }}</option><option v-for="state in states" :key="state" :value="state">{{ $t(`imports.state.${state}`) }}</option></select></label>
      <AppButton variant="secondary" :busy="task.busy" @click="refresh">{{ $t('imports.refresh') }}</AppButton>
      <AppButton :disabled="!selected.length || task.busy" @click="acceptSelection">{{ $t('imports.addSelected', { count: selected.length }) }}</AppButton>
    </div>
    <p v-if="task.error" role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
    <p v-if="added !== undefined" role="status">{{ $t('imports.batchResult', { added, failed: failures.length }) }}</p>
    <p v-for="failure in failures" :key="failure.id" class="import-error">{{ failure.name }}: {{ $t(importErrorKey(failure.cause)) }}</p>
    <p v-if="!visibleRecords.length && !task.busy">{{ $t('imports.noResults') }}</p>
    <ul class="import-list">
      <li v-for="item in visibleRecords" :key="item.id">
        <label class="import-selection"><input type="checkbox" :aria-label="$t('imports.selectFile', { name: item.originalName })" :checked="selected.some(value => value.id === item.id)" :disabled="item.state !== 'ready' || task.busy" @change="toggle(item, ($event.target as HTMLInputElement).checked)"></label>
        <div class="import-copy"><strong>{{ item.originalName }}</strong><span>{{ $t(`imports.state.${item.state}`) }}</span></div>
        <RouterLink v-if="item.libraryId && item.state !== 'removing'" class="app-button app-button--secondary import-read" :to="{ name: 'reader', params: { bookId: item.libraryId } }">{{ $t('imports.flow.read') }}</RouterLink>
        <AppButton v-else variant="secondary" @click="$emit('review', item.id)">{{ $t('imports.flow.checkBook') }}</AppButton>
      </li>
    </ul>
    <div v-if="after || next" class="app-actions import-actions">
      <AppButton variant="secondary" :disabled="!after || task.busy" @click="page('')">{{ $t('imports.first') }}</AppButton>
      <AppButton variant="secondary" :disabled="!next || task.busy" @click="page(next)">{{ $t('imports.next') }}</AppButton>
    </div>
  </section>
</template>
