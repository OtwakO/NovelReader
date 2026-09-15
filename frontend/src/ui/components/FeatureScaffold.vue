<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({
  name: 'FeatureScaffold',
  props: {
    eyebrow: { type: String, default: '' },
    title: { type: String, required: true },
    description: { type: String, required: true },
  },
});
</script>

<template>
  <section class="feature">
    <header>
      <div class="feature-heading"><h1>{{ title }}</h1><div v-if="$slots.actions" class="app-actions"><slot name="actions" /></div></div>
      <p v-if="description" class="feature-description">{{ description }}</p>
      <p v-if="eyebrow" class="feature-context">{{ eyebrow }}</p>
    </header>
    <slot />
  </section>
</template>

<style scoped>
.feature { width: min(100%, 76rem); min-width: 0; margin-inline: auto; }
header { margin-bottom: var(--space-6); }
.feature-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: var(--space-3); }
h1 { margin: 0; font: var(--weight-strong) var(--text-page)/var(--line-heading) var(--font-ui); }
.feature-description { max-width: 65ch; margin: var(--space-2) 0 0; color: var(--color-ink-muted); font-size: var(--text-small); }
.feature-context { margin: var(--space-2) 0 0; color: var(--color-ink-muted); font-size: var(--text-caption); }
</style>
