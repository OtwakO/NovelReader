<script lang="ts">
import { defineComponent } from 'vue';
import { RouterView } from 'vue-router';
import LocaleSwitcher from '../../ui/components/LocaleSwitcher.vue';

export default defineComponent({
  name: 'PublicLayout',
  components: { RouterView, LocaleSwitcher },
});
</script>

<template>
  <main class="public-shell">
    <section class="public-brand" aria-label="NovelReader">
      <LocaleSwitcher class="language" />
      <p class="eyebrow">NovelReader</p>
      <h1>{{ $t('app.brand.tagline') }}</h1>
      <p>{{ $t('app.brand.description') }}</p>
    </section>
    <section class="public-panel">
      <RouterView />
    </section>
  </main>
</template>

<style scoped>
.public-shell { min-height: 100dvh; display: grid; grid-template-columns: minmax(17rem, .85fr) minmax(20rem, 1.15fr); background: var(--color-paper); }
.public-brand { position: relative; display: flex; flex-direction: column; justify-content: flex-end; padding: clamp(2rem, 7vw, 6rem); background: linear-gradient(145deg, var(--color-accent-strong), var(--color-accent)); color: white; }
.language { position: absolute; inset: 1rem 1rem auto auto; }
.public-brand h1 { max-width: 8em; margin: .5rem 0 1rem; font: 700 clamp(2.5rem, 6vw, 5.5rem)/1.05 var(--font-literary); }
.public-brand p { max-width: 30rem; line-height: 1.7; }
.eyebrow { font-size: .78rem; font-weight: 800; letter-spacing: .16em; text-transform: uppercase; }
.public-panel { display: grid; place-items: center; padding: clamp(1rem, 5vw, 4rem); }
@media (max-width: 48rem) { .public-shell { grid-template-columns: 1fr; } .public-brand { min-height: 13rem; justify-content: center; padding: 1.5rem; } .public-brand h1 { font-size: 2.25rem; } .public-panel { align-items: start; padding: 1rem; } }
</style>
