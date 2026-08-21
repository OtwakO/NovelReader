<script lang="ts">
import { defineComponent } from "vue";

export default defineComponent({
  name: "BookDetailSection",
  props: {
    title: { type: String, required: true },
  },
});
</script>

<template>
  <section class="book-detail-section">
    <header class="book-detail-section__header">
      <div>
        <h2>{{ title }}</h2>
        <slot name="summary" />
      </div>
      <slot name="status" />
    </header>
    <slot />
  </section>
</template>

<style scoped>
.book-detail-section {
  margin-top: 1rem;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-paper-raised);
}
.book-detail-section__header {
  min-height: 4.5rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.9rem 1rem;
  border-bottom: 1px solid var(--color-border);
}
.book-detail-section__header h2 {
  margin: 0;
  font: 700 1.15rem var(--font-literary);
}
.book-detail-section__header :deep(p) {
  margin: 0;
  color: var(--color-ink-muted);
  font-size: 0.8rem;
}
.book-detail-section__header > :deep(*) {
  flex: 0 0 auto;
}
@media (max-width: 42rem) {
  .book-detail-section__header {
    align-items: baseline;
  }
}
</style>
