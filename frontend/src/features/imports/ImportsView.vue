<script lang="ts">
import { defineComponent } from 'vue';
import AppButton from '../../ui/components/AppButton.vue';
import ImportQueuePanel from './ImportQueuePanel.vue';
import ImportReceiptsPanel from './ImportReceiptsPanel.vue';
import ImportInboxPanel from './ImportInboxPanel.vue';
import './imports.css';
export default defineComponent({
  components: { AppButton, ImportQueuePanel, ImportReceiptsPanel, ImportInboxPanel },
  data: () => ({ section: 'results' as 'results' | 'inbox' }),
});
</script>

<template>
  <div class="imports-page">
    <header><h1>{{ $t('imports.title') }}</h1><p>{{ $t('imports.intro') }}</p></header>
    <ImportQueuePanel />
    <nav class="import-actions" :aria-label="$t('imports.sections')">
      <AppButton :variant="section === 'results' ? 'primary' : 'secondary'" :aria-pressed="section === 'results'" @click="section = 'results'">{{ $t('imports.results') }}</AppButton>
      <AppButton :variant="section === 'inbox' ? 'primary' : 'secondary'" :aria-pressed="section === 'inbox'" @click="section = 'inbox'">{{ $t('imports.inbox') }}</AppButton>
    </nav>
    <ImportReceiptsPanel v-show="section === 'results'" />
    <!-- Opening Imports scans the inbox; switching sections does not remount it. -->
    <ImportInboxPanel v-show="section === 'inbox'" />
  </div>
</template>
