<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({
  name: 'FeatureScaffold',
  props: {
    eyebrow: { type: String, default: 'NovelReader' },
    title: { type: String, required: true },
    description: { type: String, required: true },
  },
});
</script>

<template>
  <section class="feature">
    <header><p>{{ eyebrow }}</p><h1>{{ title }}</h1><div>{{ description }}</div></header>
    <slot />
  </section>
</template>

<style scoped>
.feature { width: min(100%, 76rem); min-width: 0; margin-inline: auto; }
header { margin-bottom: 1.5rem; }
header p { margin: 0 0 .35rem; color: var(--color-warm); font-size: .78rem; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
h1 { margin: 0; font: 700 clamp(2rem, 5vw, 3.5rem)/1.12 var(--font-literary); }
header div { max-width: 45rem; margin-top: .65rem; color: var(--color-ink-muted); line-height: 1.65; }
</style>
