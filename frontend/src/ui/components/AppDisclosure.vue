<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({
  name: 'AppDisclosure',
  props: { open: Boolean },
  emits: ['update:open', 'toggle'],
  methods: {
    toggled(event: Event) {
      this.$emit('update:open', (event.target as HTMLDetailsElement).open);
      this.$emit('toggle', event);
    },
  },
});
</script>

<template>
  <details class="app-disclosure" :open="open" @toggle="toggled">
    <summary>
      <span class="app-disclosure__label"><slot name="summary" /></span>
      <svg viewBox="0 0 20 20" aria-hidden="true"><path d="m5 7 5 6 5-6" /></svg>
    </summary>
    <div class="app-disclosure__body"><slot /></div>
  </details>
</template>

<style scoped>
.app-disclosure { min-width: 0; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-paper-raised); }
summary { min-height: var(--control-height); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); padding: var(--space-3) var(--space-4); color: var(--color-ink-muted); cursor: pointer; font-size: var(--text-small); font-weight: var(--weight-strong); line-height: var(--line-ui); list-style: none; }
summary::-webkit-details-marker { display: none; }
.app-disclosure__label { min-width: 0; overflow-wrap: anywhere; }
summary svg { width: 1.125rem; height: 1.125rem; flex: none; fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.7; transition: transform .18s ease-out; }
.app-disclosure[open] > summary { background: var(--color-paper); color: var(--color-ink); border-bottom: 1px solid var(--color-border); border-radius: var(--radius-md) var(--radius-md) 0 0; }
.app-disclosure[open] > summary svg { transform: rotate(180deg); }
summary:focus-visible { outline: 2px solid var(--color-accent); outline-offset: -2px; border-radius: var(--radius-md); }
.app-disclosure__body { padding: var(--space-4); }
.app-disclosure__body > :deep(:first-child) { margin-top: 0; }
.app-disclosure__body > :deep(:last-child) { margin-bottom: 0; }
@media (prefers-reduced-motion: reduce) { summary svg { transition: none; } }
</style>
