<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({
  name: 'BookCover',
  props: {
    name: { type: String, required: true },
    url: { type: String, default: '' },
    alt: { type: String, default: '' },
    lazy: { type: Boolean, default: false },
  },
  data() { return { failed: false, needsBackdrop: false }; },
  watch: {
    url() {
      this.failed = false;
      this.needsBackdrop = false;
    },
  },
  methods: {
    imageLoaded(event: Event) {
      const image = event.currentTarget as HTMLImageElement;
      const ratio = image.naturalWidth / image.naturalHeight;
      this.needsBackdrop = Number.isFinite(ratio) && Math.abs(ratio / (3 / 4) - 1) > 0.05;
    },
  },
});
</script>

<template>
  <div class="book-cover" :class="{ 'has-backdrop': needsBackdrop }">
    <span class="cover-fallback" aria-hidden="true">{{ name.slice(0, 1) }}</span>
    <img v-if="url && !failed && needsBackdrop" class="cover-backdrop" :src="url" alt="" aria-hidden="true">
    <img v-if="url && !failed" class="cover-image" :src="url" :alt="alt" :loading="lazy ? 'lazy' : undefined" @load="imageLoaded" @error="failed = true">
  </div>
</template>

<style scoped>
.book-cover { position: relative; isolation: isolate; width: 100%; aspect-ratio: 3 / 4; display: grid; place-items: center; overflow: hidden; background: linear-gradient(145deg,var(--color-accent),var(--color-accent-strong)); color: white; font: 700 2.5rem var(--font-literary); }
.book-cover img { position: absolute; width: 100%; height: 100%; }
.cover-fallback { z-index: 0; }
.cover-backdrop { z-index: 1; inset: -12%; width: 124% !important; height: 124% !important; object-fit: cover; filter: blur(14px) saturate(.55) brightness(.68); opacity: .72; }
.book-cover.has-backdrop::after { position: absolute; z-index: 2; inset: 0; background: color-mix(in srgb,var(--color-paper) 28%,transparent); content: ""; }
.cover-image { z-index: 3; inset: 0; object-fit: contain; }
</style>
