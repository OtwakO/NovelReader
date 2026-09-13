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
  data: () => ({
    queue: useImportQueue(), task: createImportTask(), records: [] as TXTReceipt[], next: '',
    selected: [] as TXTReceipt[], confirming: false, added: undefined as number | undefined,
    failures: [] as { id: string; name: string; cause: unknown }[], timer: undefined as ReturnType<typeof setInterval> | undefined,
    states: ['receiving', 'received', 'analyzing', 'ready', 'needs_review', 'analysis_failed', 'failed', 'removing', 'published'],
  }),
  computed: {
    after(): string { return typeof this.$route.query.after === 'string' ? this.$route.query.after : ''; },
    filter(): string { return typeof this.$route.query.state === 'string' ? this.$route.query.state : ''; },
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
    changePage() { this.task.cancel(); this.selected = []; this.confirming = false; this.refresh(); },
    page(cursor: string, state?: string) { void this.$router.push({ path: '/imports', query: { after: cursor || undefined, state: (state ?? this.filter) || undefined } }); },
    toggle(item: TXTReceipt, checked: boolean) {
      this.selected = this.selected.filter(value => value.id !== item.id);
      if (checked) this.selected.push({ ...item }); // Retain the selected version, never silently approve a newer one.
      this.confirming = false;
    },
    acceptSelection() {
      const batch = [...this.selected];
      void this.task.run(async signal => {
        this.added = 0; this.failures = []; this.confirming = false;
        for (const item of batch) {
          try {
            await acceptTXT(item.id, item.analysisVersion, importedTitle(item.originalName), '', signal);
            signal.throwIfAborted(); this.added++;
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
    <div class="import-actions">
      <label>{{ $t('imports.filter') }} <select :value="filter" :disabled="task.busy" @change="page('', ($event.target as HTMLSelectElement).value)"><option value="">{{ $t('imports.all') }}</option><option v-for="state in states" :key="state" :value="state">{{ $t(`imports.state.${state}`) }}</option></select></label>
      <AppButton variant="secondary" :busy="task.busy" @click="refresh">{{ $t('imports.refresh') }}</AppButton>
      <AppButton :disabled="!selected.length || task.busy" @click="confirming = true">{{ $t('imports.addSelected', { count: selected.length }) }}</AppButton>
    </div>
    <p v-if="task.error" role="alert" class="import-error">{{ $t(importErrorKey(task.error)) }}</p>
    <p v-if="added !== undefined" role="status">{{ $t('imports.batchResult', { added, failed: failures.length }) }}</p>
    <p v-for="failure in failures" :key="failure.id" class="import-error">{{ failure.name }}: {{ $t(importErrorKey(failure.cause)) }}</p>
    <div v-if="confirming" class="import-confirmation">
      <p>{{ $t('imports.bulkConfirm', { count: selected.length }) }}</p>
      <AppButton :disabled="task.busy" @click="acceptSelection">{{ $t('imports.confirmAdd') }}</AppButton>
      <AppButton variant="quiet" @click="confirming = false">{{ $t('imports.cancel') }}</AppButton>
    </div>
    <p v-if="!records.length && !task.busy">{{ $t('imports.noResults') }}</p>
    <ul class="import-list">
      <li v-for="item in records" :key="item.id">
        <label class="import-selection"><input type="checkbox" :aria-label="$t('imports.selectFile', { name: item.originalName })" :checked="selected.some(value => value.id === item.id)" :disabled="item.state !== 'ready' || task.busy" @change="toggle(item, ($event.target as HTMLInputElement).checked)"></label>
        <div class="import-copy"><strong>{{ item.originalName }}</strong><span>{{ $t(`imports.state.${item.state}`) }}</span></div>
        <RouterLink v-if="item.libraryId && item.state !== 'removing'" :to="`/books/${item.libraryId}`">{{ $t('imports.openBook') }}</RouterLink>
        <RouterLink v-else :to="{ path: `/imports/${item.id}`, query: $route.query }">{{ $t('imports.review') }}</RouterLink>
      </li>
    </ul>
    <div class="import-actions">
      <AppButton variant="secondary" :disabled="!after || task.busy" @click="page('')">{{ $t('imports.first') }}</AppButton>
      <AppButton variant="secondary" :disabled="!next || task.busy" @click="page(next)">{{ $t('imports.next') }}</AppButton>
    </div>
  </section>
</template>
