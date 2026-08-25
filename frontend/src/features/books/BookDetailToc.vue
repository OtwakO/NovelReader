<script lang="ts">
import { defineComponent, type PropType } from "vue";
import type { Chapter } from "../../api/models";
import AppButton from "../../ui/components/AppButton.vue";
import TocChapterList from "../reader/TocChapterList.vue";
import BookDetailSection from "./BookDetailSection.vue";
import {
  readableChapterCount,
  visibleTocChapters,
  type TocOrder,
} from "../reader/reader-toc";

export default defineComponent({
  name: "BookDetailToc",
  components: { AppButton, BookDetailSection, TocChapterList },
  props: {
    bookId: { type: String, required: true },
    chapters: { type: Array as PropType<Chapter[]>, default: () => [] },
    currentIndex: { type: Number, required: true },
    error: { type: String, default: "" },
    interactive: { type: Boolean, default: true },
  },
  data() {
    return { query: "", order: "ascending" as TocOrder, expanded: false };
  },
  computed: {
    readableCount(): number {
      return readableChapterCount(this.chapters);
    },
    summaryKey(): string {
      return this.readableCount === this.chapters.length
        ? "reader.toc.readableSummary"
        : "reader.toc.summary";
    },
    filteredChapters(): Chapter[] {
      return visibleTocChapters(this.chapters, this.query, this.order);
    },
    visibleChapters(): Chapter[] {
      return this.expanded || this.query
        ? this.filteredChapters
        : this.filteredChapters.slice(0, 80);
    },
    canExpand(): boolean {
      return (
        !this.query &&
        !this.expanded &&
        this.filteredChapters.length > this.visibleChapters.length
      );
    },
  },
  methods: {
    toggleOrder() {
      this.order = this.order === "ascending" ? "descending" : "ascending";
    },
    clearSearch() {
      this.query = "";
    },
    jumpToCurrent() {
      this.query = "";
      this.expanded = true;
      this.$nextTick(() =>
        document
          .querySelector<HTMLElement>(
            '.book-detail-toc [aria-current="location"]',
          )
          ?.scrollIntoView({ block: "center", behavior: "smooth" }),
      );
    },
  },
});
</script>

<template>
  <BookDetailSection class="book-detail-toc" :title="$t('bookDetail.chapters')">
    <template #summary>
<p>
        {{
          $t(summaryKey, { readable: readableCount, total: chapters.length })
        }}
      </p>
</template>
    <template v-if="query" #status>
<span class="toc-status">{{
        $t("reader.toc.matches", { count: filteredChapters.length })
      }}</span>
</template>
    <p v-if="error" class="banner-error" role="alert">{{ error }}</p>
    <template v-else-if="chapters.length">
      <div class="toc-tools">
        <label><span>{{ $t("reader.toc.search") }}</span><span class="search-input"><input
              v-model="query"
              type="search"
              :placeholder="$t('reader.toc.searchPlaceholder')"
            ><button
              v-if="query"
              type="button"
              :aria-label="$t('reader.toc.clearSearch')"
              @click="clearSearch"
            >
              ×
            </button></span></label>
        <div>
          <AppButton variant="secondary" @click="toggleOrder">
{{
            order === "ascending"
              ? $t("reader.toc.descending")
              : $t("reader.toc.ascending")
          }}
          </AppButton><AppButton v-if="interactive" variant="secondary" @click="jumpToCurrent">
{{
            $t("reader.toc.jumpCurrent")
          }}
</AppButton>
        </div>
      </div>
      <TocChapterList v-if="visibleChapters.length" :chapters="visibleChapters" :current-index="currentIndex" :book-id="bookId" :interactive="interactive" />
      <section v-else class="no-matches">
        <p>{{ $t("reader.toc.noMatches") }}</p>
        <AppButton variant="secondary" @click="clearSearch">
{{
          $t("reader.toc.clearSearch")
        }}
</AppButton>
      </section>
      <AppButton v-if="canExpand" variant="quiet" @click="expanded = true">
{{
        $t("bookDetail.showAll", { count: filteredChapters.length })
      }}
</AppButton>
    </template>
    <p v-else class="muted">{{ $t("bookDetail.noChapters") }}</p>
  </BookDetailSection>
</template>

<style scoped>
.toc-status {
  color: var(--color-ink-muted);
  font-size: 0.8rem;
}
.toc-tools {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: end;
  padding: 0.85rem 1rem;
  border-bottom: 1px solid var(--color-border);
  background: var(--color-paper-raised);
}
.toc-tools label {
  display: grid;
  gap: 0.3rem;
}
.toc-tools label > span:first-child {
  color: var(--color-ink-muted);
  font-size: 0.75rem;
  font-weight: 700;
}
.search-input {
  position: relative;
}
.search-input input {
  width: 100%;
  min-height: 2.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0.6rem 2.75rem 0.6rem 0.75rem;
  background: white;
  color: var(--color-ink);
}
.search-input button {
  position: absolute;
  right: 0.25rem;
  top: 0.25rem;
  width: 2.25rem;
  height: 2.25rem;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-ink-muted);
  font-size: 1.25rem;
}
.toc-tools > div {
  display: flex;
  gap: 0.5rem;
}
.no-matches {
  display: grid;
  justify-items: center;
  gap: 0.75rem;
  padding: 2rem;
  text-align: center;
}
.muted,
.no-matches p {
  color: var(--color-ink-muted);
}
.muted,
.banner-error {
  margin: 1rem;
}
.banner-error {
  padding: 0.7rem;
  border-radius: var(--radius-md);
  background: #f8e4df;
  color: var(--color-danger);
}
.book-detail-toc > :deep(.app-button) {
  margin: 0.75rem 1rem 1rem;
}
.toc-tools :deep(.app-button) {
  margin: 0;
}
@media (max-width: 42rem) {
  .toc-tools {
    grid-template-columns: 1fr;
  }
  .toc-tools > div {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
  .toc-tools :deep(.app-button) {
    width: 100%;
  }
}
</style>
