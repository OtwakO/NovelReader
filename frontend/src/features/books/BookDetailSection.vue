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
      <div v-if="$slots.status || $slots.actions" class="app-actions book-detail-section__actions">
        <slot name="status" />
        <slot name="actions" />
      </div>
    </header>
    <div v-if="$slots.body" class="book-detail-section__body">
      <slot name="body" />
    </div>
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
  flex-wrap: wrap;
  padding: 0.9rem 1rem;
  border-bottom: 1px solid var(--color-border);
}
.book-detail-section__header h2 {
  margin: 0;
  font: var(--weight-strong) var(--text-section) var(--font-literary);
}
.book-detail-section__header :deep(p) {
  margin: 0;
  color: var(--color-ink-muted);
  font-size: var(--text-caption);
}
.book-detail-section__header > div:first-child { min-width: 0; }
.book-detail-section__actions { margin-inline-start: auto; justify-content: flex-end; }
.book-detail-section__body {
  padding: 1rem;
}
.book-detail-section__body :deep(> :first-child) {
  margin-top: 0;
}
.book-detail-section__body :deep(> :last-child) {
  margin-bottom: 0;
}
</style>
