<script lang="ts">
import AppDisclosure from '../../../ui/components/AppDisclosure.vue';
import { defineComponent, type PropType } from 'vue';
import type { SearchIntensity } from '../search-preferences';

export default defineComponent({
  name: 'SearchControls',
  components: { AppDisclosure },
  props: {
    batchSize: { type: Number, required: true },
    intensity: { type: String as PropType<SearchIntensity>, required: true },
    advancedConcurrency: { type: Number, required: true },
  },
  emits: ['update:batchSize', 'update:intensity', 'update:advancedConcurrency', 'change'],
  methods: {
    numberValue(event: Event) { return Number((event.target as HTMLInputElement).value); },
    stringValue(event: Event) { return (event.target as HTMLSelectElement).value as SearchIntensity; },
  },
});
</script>

<template>
  <AppDisclosure class="controls">
    <template #summary>{{ $t('search.controls.title') }}</template>
    <div class="controls-grid">
      <label><span>{{ $t('search.controls.batchSize') }}</span><input type="number" min="1" max="500" :value="batchSize" @change="$emit('update:batchSize', numberValue($event)); $emit('change')"></label>
      <label><span>{{ $t('search.controls.intensity') }}</span><select :value="intensity" @change="$emit('update:intensity', stringValue($event)); $emit('change')"><component :is="'button'" type="button"><selectedcontent /></component><option value="gentle">{{ $t('search.controls.gentle') }}</option><option value="balanced">{{ $t('search.controls.balanced') }}</option><option value="fast">{{ $t('search.controls.fast') }}</option><option value="advanced">{{ $t('search.controls.advanced') }}</option></select></label>
      <label v-if="intensity === 'advanced'"><span>{{ $t('search.controls.concurrency') }}</span><input type="number" min="1" :value="advancedConcurrency" @change="$emit('update:advancedConcurrency', numberValue($event)); $emit('change')"></label>
    </div>
  </AppDisclosure>
</template>

<style scoped>
.controls-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr)); gap: var(--space-3); }
label { display: grid; gap: .35rem; } label span { color: var(--color-ink-muted); font-size: var(--text-small); font-weight: var(--weight-strong); }
input, select { min-height: 2.75rem; width: 100%; border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: .55rem .7rem; background: white; color: var(--color-ink); }
</style>
