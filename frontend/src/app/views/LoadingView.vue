<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({ name: 'LoadingView' });
</script>

<template>
  <main class="loading" aria-busy="true" aria-live="polite">
    <span class="mark" aria-hidden="true">N</span>
    <p>{{ $t('app.startup.opening') }}</p>
  </main>
</template>

<style scoped>
.loading { min-height: 100dvh; display: grid; place-content: center; justify-items: center; gap: 1rem; background: var(--color-paper); color: var(--color-ink-muted); }
.mark { width: 3.5rem; height: 3.5rem; display: grid; place-items: center; border-radius: 50%; background: var(--color-accent); color: white; font: 700 1.5rem var(--font-literary); }
p { margin: 0; }
</style>
