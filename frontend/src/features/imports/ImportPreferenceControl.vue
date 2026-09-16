<script lang="ts">
import { defineComponent } from 'vue';
import { useImportPreferences } from './import-preferences';
export default defineComponent({
  data: () => ({ preferences: useImportPreferences() }),
});
</script>
<template>
  <label class="import-preference">
    <input v-model="preferences.reviewBeforeAdding" type="checkbox">
    <span><strong>{{ $t('imports.preferences.review') }}</strong><small>{{ $t('imports.preferences.hint') }}</small></span>
  </label>
</template>
<style scoped>
.import-preference { display: flex; align-items: flex-start; gap: var(--space-3); padding-block: var(--space-3); cursor: pointer; }
.import-preference input { flex: 0 0 auto; margin-top: .35rem; }
.import-preference span { min-width: 0; }
.import-preference strong { display: block; font-size: var(--text-body); font-weight: var(--weight-regular); }
.import-preference small { display: block; margin-top: var(--space-1); color: var(--color-ink-muted); font-size: var(--text-small); }
</style>
