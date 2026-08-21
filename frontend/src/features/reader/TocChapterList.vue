<script lang="ts">
import { defineComponent, type PropType } from "vue";
import type { Chapter } from "../../api/models";

export default defineComponent({
  name: "TocChapterList",
  props: {
    chapters: { type: Array as PropType<Chapter[]>, default: () => [] },
    currentIndex: { type: Number, required: true },
    bookId: { type: String, default: "" },
  },
  emits: ["open"],
});
</script>

<template>
  <ol class="toc-chapter-list">
    <li
      v-for="chapter in chapters"
      :key="chapter.index"
      :data-current="chapter.index === currentIndex"
      :class="{ volume: chapter.isVolume, current: chapter.index === currentIndex }"
    >
      <span v-if="chapter.isVolume" class="volume-title">{{ chapter.title }}</span>
      <RouterLink
        v-else-if="bookId"
        :to="`/books/${encodeURIComponent(bookId)}/read/${chapter.index}`"
        :aria-current="chapter.index === currentIndex ? 'location' : undefined"
      >
        <small>{{ chapter.index + 1 }}</small><span class="chapter-title">{{ chapter.title }}</span><svg class="row-arrow" viewBox="0 0 20 20" aria-hidden="true"><path d="m7 4 6 6-6 6" /></svg>
      </RouterLink>
      <button
        v-else
        type="button"
        :aria-current="chapter.index === currentIndex ? 'location' : undefined"
        @click="$emit('open', chapter.index)"
      >
        <small>{{ chapter.index + 1 }}</small><span class="chapter-title">{{ chapter.title }}</span><svg class="row-arrow" viewBox="0 0 20 20" aria-hidden="true"><path d="m7 4 6 6-6 6" /></svg>
      </button>
    </li>
  </ol>
</template>

<style scoped>
.toc-chapter-list {
  margin: 0;
  padding: 0;
  list-style: none;
}
.toc-chapter-list li {
  position: relative;
  min-width: 0;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 72%, transparent);
}
.toc-chapter-list li:last-child {
  border-bottom: 0;
}
.toc-chapter-list a,
.toc-chapter-list button {
  width: 100%;
  min-height: 3.5rem;
  display: grid;
  grid-template-columns: 3.4rem minmax(0, 1fr) 1.25rem;
  align-items: center;
  gap: 0.9rem;
  border: 0;
  padding: 0.75rem 1rem;
  background: transparent;
  color: var(--color-ink);
  text-align: left;
  text-decoration: none;
  cursor: pointer;
  transition: background 0.18s ease-out, color 0.18s ease-out;
}
.toc-chapter-list a:hover,
.toc-chapter-list button:hover {
  background: color-mix(in srgb, var(--color-accent-soft) 44%, transparent);
}
.toc-chapter-list a:focus-visible,
.toc-chapter-list button:focus-visible {
  position: relative;
  z-index: 1;
  outline: 2px solid var(--color-focus);
  outline-offset: -2px;
}
.toc-chapter-list small {
  color: var(--color-ink-muted);
  font-size: 0.72rem;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.04em;
  text-align: right;
}
.chapter-title {
  min-width: 0;
  overflow-wrap: anywhere;
  line-height: 1.5;
}
.row-arrow {
  width: 1rem;
  height: 1rem;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.7;
  opacity: 0;
  transform: translateX(-0.2rem);
  transition: opacity 0.18s ease-out, transform 0.18s ease-out;
}
.toc-chapter-list a:hover .row-arrow,
.toc-chapter-list button:hover .row-arrow,
.toc-chapter-list a:focus-visible .row-arrow,
.toc-chapter-list button:focus-visible .row-arrow {
  opacity: 0.65;
  transform: translateX(0);
}
.current a,
.current button {
  background: color-mix(in srgb, var(--color-accent-soft) 72%, var(--color-paper-raised));
  color: var(--color-accent-strong);
  font-weight: 700;
}
.current small {
  color: var(--color-warm);
  font-weight: 800;
}
.current .row-arrow {
  color: inherit;
}
.current .row-arrow {
  opacity: 0.55;
  transform: none;
}
.volume-title {
  min-height: 2.5rem;
  display: flex;
  align-items: center;
  padding: 0.65rem 1rem;
  background: color-mix(in srgb, var(--color-paper-muted) 58%, var(--color-paper-raised));
  color: var(--color-ink-muted);
  font-size: 0.76rem;
  font-weight: 800;
  letter-spacing: 0.035em;
}
@media (max-width: 32rem) {
  .toc-chapter-list a,
  .toc-chapter-list button {
    grid-template-columns: 2.75rem minmax(0, 1fr) 1rem;
    gap: 0.7rem;
    padding-inline: 0.75rem;
  }
}
</style>
