<script lang="ts">
import { defineComponent } from "vue";
import {
  clearBookSources,
  deleteBook,
  getBook,
  mergeBookSources,
  type Book,
} from "../../api/books";
import type { AltSource, Chapter } from "../../api/models";
import { switchBookSource, waitForCatalog } from "../../api/reader";
import AppButton from "../../ui/components/AppButton.vue";
import FeatureScaffold from "../../ui/components/FeatureScaffold.vue";
import SourceRecoveryPanel from "../source-recovery/SourceRecoveryPanel.vue";
import BookCover from "./BookCover.vue";
import BookDetailSection from "./BookDetailSection.vue";
import BookDetailToc from "./BookDetailToc.vue";
import { clearCandidateCommittedBook } from "../candidates/candidate-operation";
import { readableChapterLabel } from "./book-display";

export default defineComponent({
  name: "BookDetailView",
  components: {
    AppButton,
    BookCover,
    BookDetailSection,
    BookDetailToc,
    FeatureScaffold,
    SourceRecoveryPanel,
  },
  data() {
    return {
      book: null as Book | null,
      chapters: [] as Chapter[],
      loading: true,
      bookError: "",
      tocError: "",
      catalogSyncing: false,
      catalogRetrying: false,
      loadGeneration: 0,
      sourceError: "",
      sourceMessage: "",
      switching: false,
      removing: false,
      confirmingRemove: false,
      persistence: Promise.resolve() as Promise<void>,
    };
  },
  computed: {
    bookId(): string {
      return String(this.$route.params.bookId || "");
    },
    displayLastChapter(): string {
      return readableChapterLabel(this.book?.lastChapter);
    },
    progress(): number {
      if (!this.book?.totalChapterNum) return 0;
      return Math.min(
        100,
        Math.round(
          ((this.book.durChapterIndex + this.book.durChapterPos) /
            this.book.totalChapterNum) *
            100,
        ),
      );
    },
  },
  watch: {
    bookId() {
      this.book = null;
      this.chapters = [];
      void this.load();
    },
  },
  async mounted() {
    await this.load();
  },
  methods: {
    async load() {
      const request = ++this.loadGeneration;
      this.loading = true;
      this.bookError = "";
      this.tocError = "";
      try {
        const book = await getBook(this.bookId);
        if (request !== this.loadGeneration) return;
        this.book = book;
        this.loading = false;
        await this.loadCatalog(false, request);
      } catch (cause) {
        if (request !== this.loadGeneration) return;
        this.bookError =
          cause instanceof Error
            ? cause.message
            : this.$t("bookDetail.loadFailed");
        this.loading = false;
      }
    },
    async loadCatalog(retry = false, request?: number) {
      request ??= this.loadGeneration;
      this.catalogSyncing = true;
      this.catalogRetrying = retry;
      this.tocError = "";
      try {
        const chapters = await waitForCatalog(this.bookId, {
          retry,
          isCurrent: () => request === this.loadGeneration,
        });
        if (request === this.loadGeneration) this.chapters = chapters;
      } catch (cause) {
        if (request !== this.loadGeneration) return;
        this.tocError =
          cause instanceof Error
            ? cause.message
            : this.$t("bookDetail.tocFailed");
      } finally {
        if (request === this.loadGeneration) {
          this.catalogSyncing = false;
          this.catalogRetrying = false;
        }
      }
    },
    persistMatches(sources: AltSource[]) {
      if (!sources.length || !this.book) return;
      this.persistence = this.persistence
        .then(async () => {
          if (this.book)
            this.book = await mergeBookSources(this.book.id, sources);
        })
        .catch((cause) => {
          this.sourceError =
            cause instanceof Error
              ? cause.message
              : this.$t("sourceRecovery.persistFailed");
        });
    },
    async clearAndRescan() {
      try {
        await this.persistence;
        if (!this.book) throw new Error(this.$t("bookDetail.notFound"));
        this.book = await clearBookSources(this.book.id);
        this.sourceMessage = this.$t("sourceRecovery.cleared");
        this.sourceError = "";
      } catch (cause) {
        this.sourceError =
          cause instanceof Error
            ? cause.message
            : this.$t("sourceRecovery.clearFailed");
        throw cause;
      }
    },
    async selectSource(source: AltSource) {
      if (!this.book || this.switching) return;
      this.switching = true;
      this.sourceError = "";
      this.sourceMessage = "";
      try {
        this.book = await mergeBookSources(this.book.id, [source]);
        const result = await switchBookSource(
          this.book.id,
          source.sourceId,
          source.sourceUrl,
          source.bookUrl,
        );
        this.book = result.book;
        this.loadGeneration += 1;
        this.chapters = [];
        this.tocError = "";
        this.sourceMessage =
          result.mapping === "title"
            ? this.$t("sourceRecovery.switchedTitle")
            : this.$t("sourceRecovery.switchedIndex");
        await this.loadCatalog();
      } catch (cause) {
        this.sourceError =
          cause instanceof Error
            ? cause.message
            : this.$t("sourceRecovery.switchFailed");
      } finally {
        this.switching = false;
      }
    },
    async removeBook() {
      if (!this.book || this.removing) return;
      this.removing = true;
      try {
        const bookId = this.book.id;
        await deleteBook(bookId);
        clearCandidateCommittedBook(bookId);
        await this.$router.replace("/shelf");
      } catch (cause) {
        this.bookError =
          cause instanceof Error
            ? cause.message
            : this.$t("bookDetail.removeFailed");
      } finally {
        this.removing = false;
        this.confirmingRemove = false;
      }
    },
  },
});
</script>

<template>
  <FeatureScaffold
    :title="book?.name || $t('bookDetail.title')"
    :description="$t('bookDetail.description')"
  >
    <p v-if="loading" aria-busy="true">{{ $t("bookDetail.loading") }}</p>
    <section v-else-if="!book" class="state">
      <p role="alert">{{ bookError || $t("bookDetail.notFound") }}</p>
      <RouterLink to="/shelf">{{ $t("bookDetail.back") }}</RouterLink>
    </section>
    <template v-else>
      <p v-if="bookError" class="banner-error" role="alert">{{ bookError }}</p>
      <section class="hero">
        <BookCover class="cover" :name="book.name" :url="book.coverDisplayUrl || ''" :alt="$t('bookDetail.coverAlt', { name: book.name })" />
        <div class="identity">
          <h2>{{ book.name }}</h2>
          <p class="author">
            {{ book.author || $t("app.common.unknownAuthor") }}
          </p>
          <div class="metadata">
            <span v-if="book.kind">{{ book.kind }}</span><span v-if="book.wordCount">{{ book.wordCount }}</span><span>{{
              $t("bookDetail.tocEntries", { count: chapters.length })
            }}</span><span v-if="book.totalChapterNum">{{
              $t("bookDetail.progress", { percent: progress })
            }}</span>
          </div>
          <p v-if="displayLastChapter" class="latest">
            {{ $t("bookDetail.latest", { chapter: displayLastChapter }) }}
          </p>
          <p class="source">
            {{
              $t("bookDetail.currentSource", {
                source: book.origin || book.sourceUrl,
              })
            }}
          </p>
          <div class="actions">
            <RouterLink
              class="primary-link"
              :to="`/books/${encodeURIComponent(book.id)}/read/${book.durChapterIndex}`"
              >
{{ $t("bookDetail.continue") }}
</RouterLink><AppButton variant="danger" @click="confirmingRemove = true">
{{
              $t("bookDetail.remove")
            }}
</AppButton>
          </div>
        </div>
      </section>
      <section
        v-if="confirmingRemove"
        class="confirmation"
        role="alertdialog"
        :aria-label="$t('bookDetail.confirmRemoveTitle')"
      >
        <strong>{{ $t("bookDetail.confirmRemoveTitle") }}</strong>
        <p>
          {{ $t("bookDetail.confirmRemoveDescription", { name: book.name }) }}
        </p>
        <div>
          <AppButton variant="secondary" @click="confirmingRemove = false">
{{
            $t("bookDetail.cancel")
          }}
</AppButton><AppButton variant="danger" :busy="removing" @click="removeBook">
{{
            $t("bookDetail.confirmRemove")
          }}
</AppButton>
        </div>
      </section>
      <BookDetailSection v-if="book.intro" :title="$t('bookDetail.synopsis')">
        <template #body>
          <p class="intro">{{ book.intro }}</p>
        </template>
      </BookDetailSection>
      <p v-if="catalogSyncing" class="catalog-status" role="status" aria-busy="true">
        {{ $t("bookDetail.tocSyncing") }}
      </p>
      <div v-else-if="tocError" class="catalog-failure">
        <p role="alert">{{ tocError }}</p>
        <AppButton variant="secondary" :busy="catalogRetrying" @click="loadCatalog(true)">
          {{ $t("bookDetail.retryToc") }}
        </AppButton>
      </div>
      <BookDetailToc
        :book-id="book.id"
        :chapters="chapters"
        :current-index="book.durChapterIndex"
        :error="tocError"
      />
      <SourceRecoveryPanel
        :book="book"
        :switching="switching"
        :action-error="sourceError"
        :action-message="sourceMessage"
        :on-clear-and-rescan="clearAndRescan"
        @matches="persistMatches"
        @select="selectSource"
      />
    </template>
  </FeatureScaffold>
</template>

<style scoped>
.state,
.confirmation,
.catalog-status,
.catalog-failure {
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-paper-raised);
}
.catalog-status {
  color: var(--color-ink-muted);
}
.catalog-failure {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}
.hero {
  display: grid;
  grid-template-columns: 12rem minmax(0, 1fr);
  gap: 1.5rem;
  align-items: start;
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-paper-raised);
}
.cover {
  width: 12rem;
  align-self: center;
  border-radius: 0;
}
.identity {
  min-width: 0;
}
.eyebrow {
  margin: 0;
  color: var(--color-warm);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}
.identity h2 {
  margin: 0.25rem 0;
  font: 700 clamp(1.7rem, 4vw, 2.7rem)/1.12 var(--font-literary);
}
.author,
.latest,
.source,
.muted {
  color: var(--color-ink-muted);
  overflow-wrap: anywhere;
}
.metadata,
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 0.8rem;
}
.metadata span {
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: var(--color-paper-muted);
  font-size: 0.76rem;
}
.primary-link {
  min-height: 2.75rem;
  display: inline-flex;
  align-items: center;
  border-radius: var(--radius-md);
  padding: 0.65rem 1rem;
  background: var(--color-accent);
  color: white;
  text-decoration: none;
  font-weight: 700;
}
.intro {
  width: 100%;
  margin: 0;
  line-height: 1.75;
  overflow-wrap: anywhere;
  white-space: pre-line;
}
.confirmation {
  margin-top: 1rem;
}
.confirmation p {
  color: var(--color-ink-muted);
}
.confirmation div {
  display: flex;
  gap: 0.5rem;
}
.banner-error {
  padding: 0.7rem;
  border-radius: var(--radius-md);
  background: #f8e4df;
  color: var(--color-danger);
}
:deep(.recovery) {
  margin-top: 1rem;
}
@media (max-width: 38rem) {
  .hero {
    grid-template-columns: 7rem minmax(0, 1fr);
    gap: 1rem;
  }
  .cover {
    width: 7rem;
  }
  .actions {
    grid-column: 1/-1;
  }
  .actions > * {
    flex: 1 1 10rem;
    justify-content: center;
  }
  .toc-header {
    align-items: baseline;
  }
  .chapter-list {
    grid-template-columns: minmax(0, 1fr);
  }
  .chapter-list li {
    grid-template-columns: 2.25rem minmax(0, 1fr);
    padding-inline: 0.85rem;
  }
  .chapter-list li:nth-last-child(-n + 2):not(.volume) {
    border-bottom: 1px solid var(--color-border);
  }
  .chapter-list li:last-child:not(.volume) {
    border-bottom: 0;
  }
}
</style>
