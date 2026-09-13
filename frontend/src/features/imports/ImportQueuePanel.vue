<script lang="ts">
import { defineComponent } from 'vue';
import { RouterLink } from 'vue-router';
import AppButton from '../../ui/components/AppButton.vue';
import { useImportQueue } from './import-queue';
import { importErrorKey } from './import-feedback';
export default defineComponent({
  components: { RouterLink, AppButton },
  data: () => ({ queue: useImportQueue(), offset: 0 }),
  computed: {
    visible() { return this.queue.entries.slice(this.offset, this.offset + 25); },
    active() { return this.queue.entries.find(item => ['waiting', 'transferring'].includes(item.state)); },
  },
  watch: { 'queue.entries.length'() { this.offset = Math.min(this.offset, Math.floor(Math.max(0, this.queue.entries.length - 1) / 25) * 25); } },
  methods: {
    importErrorKey,
    choose(event: Event) {
      const input = event.target as HTMLInputElement;
      this.queue.enqueue(Array.from(input.files || [])); input.value = '';
    },
  },
});
</script>

<template>
  <section aria-labelledby="upload-title" class="import-section">
    <h2 id="upload-title">{{ $t('imports.browser') }}</h2>
    <p>{{ $t('imports.browserHint') }}</p>
    <label class="file-picker">{{ $t('imports.chooseFiles') }}<input type="file" accept=".txt,text/plain" multiple @change="choose"></label>
    <template v-if="queue.entries.length">
      <div class="import-actions">
        <span role="status">{{ $t('imports.navigationStatus', { pending: queue.outstanding, attention: queue.attention }) }}</span>
        <AppButton v-if="!queue.paused" variant="secondary" :disabled="!queue.outstanding" @click="queue.pause()">{{ $t('imports.pause') }}</AppButton>
        <AppButton v-else variant="secondary" @click="queue.resume()">{{ $t('imports.resume') }}</AppButton>
        <AppButton variant="quiet" @click="queue.clearFinished(); offset = 0">{{ $t('imports.clearFinished') }}</AppButton>
      </div>
      <p v-if="queue.paused">{{ $t('imports.pauseHint') }}</p>
      <p v-if="active">{{ active.name }} — {{ $t(`imports.transfer.${active.state}`) }}</p>
      <details :open="queue.attention > 0">
        <summary>{{ $t('imports.transferDetails') }}</summary>
      <ul class="import-list">
        <li v-for="item in visible" :key="item.key">
          <div class="import-copy">
<strong>{{ item.name }}</strong><span>{{ $t(`imports.transfer.${item.state}`) }}</span>
            <p v-if="item.error" class="import-error">{{ $t(importErrorKey(item.error)) }}</p>
            <p v-for="warning in item.warnings || []" :key="warning">{{ $t('imports.retainedWarning') }} ({{ warning }})</p>
            <RouterLink v-if="item.receiptId" :to="`/imports/${item.receiptId}`">{{ $t('imports.checkReceipt') }}</RouterLink>
          </div>
          <AppButton v-if="!['waiting', 'transferring'].includes(item.state)" variant="quiet" @click="queue.remove(item.key)">{{ $t('imports.dismiss') }}</AppButton>
        </li>
      </ul>
      <div class="import-actions">
        <AppButton variant="secondary" :disabled="offset === 0" @click="offset = Math.max(0, offset - 25)">{{ $t('imports.previous') }}</AppButton>
        <AppButton variant="secondary" :disabled="offset + 25 >= queue.entries.length" @click="offset += 25">{{ $t('imports.next') }}</AppButton>
      </div>
      </details>
    </template>
  </section>
</template>
