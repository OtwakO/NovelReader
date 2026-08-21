<script lang="ts">
import { defineComponent, type PropType } from 'vue';
import type { SearchIntensity } from '../search-preferences';

export default defineComponent({
  name: 'SearchControls',
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
  <details class="controls">
    <summary>{{ $t('search.controls.title') }}<svg viewBox="0 0 20 20" aria-hidden="true"><path d="m5 7 5 6 5-6" /></svg></summary>
    <div class="controls-grid">
      <label><span>{{ $t('search.controls.batchSize') }}</span><input type="number" min="1" max="500" :value="batchSize" @change="$emit('update:batchSize', numberValue($event)); $emit('change')"></label>
      <label><span>{{ $t('search.controls.intensity') }}</span><select :value="intensity" @change="$emit('update:intensity', stringValue($event)); $emit('change')"><option value="gentle">{{ $t('search.controls.gentle') }}</option><option value="balanced">{{ $t('search.controls.balanced') }}</option><option value="fast">{{ $t('search.controls.fast') }}</option><option value="advanced">{{ $t('search.controls.advanced') }}</option></select></label>
      <label v-if="intensity === 'advanced'"><span>{{ $t('search.controls.concurrency') }}</span><input type="number" min="1" :value="advancedConcurrency" @change="$emit('update:advancedConcurrency', numberValue($event)); $emit('change')"></label>
    </div>
  </details>
</template>

<style scoped>
.controls { border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-paper-raised); }
summary { min-height: 2.75rem; display: flex; align-items: center; justify-content: space-between; gap: .75rem; padding: .6rem .85rem; color: var(--color-ink-muted); cursor: pointer; font-size: .88rem; font-weight: 700; list-style: none; }
summary::-webkit-details-marker { display: none; }
summary svg { width: 1.1rem; height: 1.1rem; flex: none; fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.7; transition: transform .18s ease-out; }
.controls[open] summary svg { transform: rotate(180deg); }
summary:focus-visible { outline: 2px solid var(--color-focus); outline-offset: -2px; }
.controls-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr)); gap: .75rem; padding: 0 .85rem .85rem; }
label { display: grid; gap: .35rem; } label span { color: var(--color-ink-muted); font-size: .78rem; font-weight: 700; }
input, select { min-height: 2.75rem; width: 100%; border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: .55rem .7rem; background: white; color: var(--color-ink); }
</style>
