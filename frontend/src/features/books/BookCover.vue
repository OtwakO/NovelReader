<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({
  name: 'BookCover',
  props: {
    name: { type: String, required: true },
    url: { type: String, default: '' },
    alt: { type: String, default: '' },
  },
  data() { return { failed: false }; },
  watch: { url() { this.failed = false; } },
});
</script>

<template>
  <div class="book-cover">
    <span aria-hidden="true">{{ name.slice(0, 1) }}</span>
    <img v-if="url && !failed" :src="url" :alt="alt" @error="failed = true">
  </div>
</template>

<style scoped>
.book-cover { position: relative; display: grid; place-items: center; overflow: hidden; background: linear-gradient(145deg,var(--color-accent),var(--color-accent-strong)); color: white; font: 700 2.5rem var(--font-literary); }
.book-cover img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; background: var(--color-paper-muted); }
</style>
