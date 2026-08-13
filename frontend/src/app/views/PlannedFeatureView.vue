<script lang="ts">
import { defineComponent } from 'vue';
import FeatureScaffold from '../../ui/components/FeatureScaffold.vue';

export default defineComponent({
  name: 'PlannedFeatureView',
  components: { FeatureScaffold },
  props: { title: { type: String, required: true }, description: { type: String, required: true } },
});
</script>

<template><FeatureScaffold :title="$t(title)" :description="$t(description)"><section class="notice"><p>{{ $t('app.planned.note') }}</p></section></FeatureScaffold></template>

<style scoped>.notice { padding: 1.25rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); color: var(--color-ink-muted); }</style>
