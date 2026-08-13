<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({
  name: 'AuthCard',
  props: {
    title: { type: String, required: true },
    intro: { type: String, default: '' },
  },
});
</script>

<template>
  <section class="auth-card">
    <h2>{{ title }}</h2>
    <p v-if="intro" class="intro">{{ intro }}</p>
    <slot />
  </section>
</template>

<style scoped>
.auth-card { width: min(100%, 31rem); padding: clamp(1.25rem, 4vw, 2rem); border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-card); }
h2 { margin: 0; font: 700 clamp(1.8rem, 5vw, 2.4rem)/1.2 var(--font-literary); }
.intro { margin: .75rem 0 1.5rem; color: var(--color-ink-muted); line-height: 1.6; }
:deep(form) { display: grid; gap: .85rem; }
:deep(label) { display: grid; gap: .4rem; font-size: .9rem; font-weight: 700; }
:deep(input), :deep(select) { width: 100%; min-height: 2.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: .65rem .75rem; background: white; color: var(--color-ink); }
:deep(.form-error) { margin: 0; padding: .75rem; border-radius: var(--radius-sm); background: #f8e4df; color: #812f27; line-height: 1.45; }
:deep(.form-actions) { display: flex; flex-wrap: wrap; gap: .5rem; margin-top: .5rem; }
</style>
