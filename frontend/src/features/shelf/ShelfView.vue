<script lang="ts">
import { defineComponent, nextTick } from "vue";
import { useImportQueue } from '../imports/import-queue';
import { readerResumeLocation } from '../reader/reader-session';
import { listBooks } from "../../api/books";
import type { LibraryBook } from "../../api/models";
import AppIcon from '../../ui/components/AppIcon.vue';
import AppButton from "../../ui/components/AppButton.vue";
import FeatureScaffold from "../../ui/components/FeatureScaffold.vue";
import BookCover from "../books/BookCover.vue";
import { readableChapterLabel } from "../books/book-display";
import { currentChapterNumber, shelfProgressPercent } from "./shelf-progress";
import { loadShelfViewState, saveShelfViewState, visibleShelfBooks, type ShelfSort } from "./shelf-view-state";

export default defineComponent({
  name: "ShelfView",
  components: { AppIcon, AppButton, BookCover, FeatureScaffold },
  data() {
    const view = loadShelfViewState();
    return { imports: useImportQueue(), loadGeneration: 0, books: [] as LibraryBook[], loading: true, error: "", query: view.query, sort: view.sort as ShelfSort, restoreScrollY: view.scrollY };
  },
  computed: {
    continueBook(): LibraryBook | null {
      return visibleShelfBooks(this.books.filter(book => (book.lastReadAt ?? 0) > 0), '', 'recent')[0] ?? null;
    },
    visibleBooks(): LibraryBook[] { return visibleShelfBooks(this.books, this.query, this.sort); },
  },
  watch: {
    'imports.libraryRevision'() { void this.load(true); },
    query() { this.saveView(); },
    sort() { this.saveView(); },
  },
  async mounted() {
    window.addEventListener('scroll', this.captureScroll, { passive: true });
    await this.load();
    await nextTick();
    window.scrollTo({ top: this.restoreScrollY });
  },
  beforeUnmount() { this.loadGeneration++; this.captureScroll(); window.removeEventListener('scroll', this.captureScroll); },
  methods: {
    readerResumeLocation,
    saveView(scrollY = window.scrollY) { saveShelfViewState({ query: this.query, sort: this.sort, scrollY }); },
    captureScroll() { this.saveView(); },
    clearQuery() { this.query = ''; },
    async load(quiet = false) {
      const generation = ++this.loadGeneration;
      if (!quiet) this.loading = true;
      this.error = "";
      try {
        const books = await listBooks();
        if (generation !== this.loadGeneration) return;
        this.books = books;
      } catch (cause) {
        if (generation !== this.loadGeneration) return;
        this.error =
          cause instanceof Error ? cause.message : this.$t("shelf.failed");
      } finally {
        if (generation === this.loadGeneration) this.loading = false;
      }
    },
    progress(book: LibraryBook) {
      return shelfProgressPercent(book);
    },
    chapter(book: LibraryBook) {
      return currentChapterNumber(book);
    },
    latestChapter(book: LibraryBook) { return readableChapterLabel(book.lastChapter); },
    currentChapter(book: LibraryBook) {
      return (
        book.currentChapterTitle ||
        this.$t("shelf.chapter", { chapter: this.chapter(book) })
      );
    },
    coverURL(book: LibraryBook) { return book.coverDisplayUrl || ''; },
  },
});
</script>

<template>
  <FeatureScaffold
    :title="$t('shelf.title')"
    :description="$t('shelf.description')"
  >
    <template #actions><RouterLink class="app-button app-button--secondary" to="/imports">{{ $t('imports.title') }}</RouterLink></template>
    <p v-if="error && books.length" role="alert">{{ error }} <AppButton variant="quiet" @click="load(true)">{{ $t('app.common.retry') }}</AppButton></p>
    <p v-if="loading" aria-busy="true">{{ $t("shelf.loading") }}</p>
    <section v-else-if="error && !books.length" class="state">
      <p role="alert">{{ error }}</p>
      <AppButton variant="secondary" @click="load()">
        {{ $t("app.common.retry") }}
      </AppButton>
    </section>
    <section v-else-if="books.length === 0" class="state">
      <h2>{{ $t("shelf.emptyTitle") }}</h2>
      <p>{{ $t("shelf.emptyDescription") }}</p>
      <div class="app-actions">
        <RouterLink class="app-button app-button--secondary" to="/explore">{{ $t("shelf.explore") }}</RouterLink><RouterLink class="app-button app-button--secondary" to="/search">{{ $t("shelf.search") }}</RouterLink>
      </div>
    </section>
    <div v-else class="library">
      <section
        v-if="continueBook"
        class="continue-section"
        :aria-labelledby="`continue-${continueBook.id}`"
      >
        <header>
          <h2>{{ $t("shelf.continueReading") }}</h2>
          <span>{{ progress(continueBook) }}%</span>
        </header>
        <div class="continue-panel">
          <RouterLink
            class="continue-cover"
            :to="`/books/${encodeURIComponent(continueBook.id)}`"
            :aria-label="$t('shelf.detailsFor', { name: continueBook.name })"
          >
            <BookCover
              :name="continueBook.name"
              :url="coverURL(continueBook)"
              :alt="$t('shelf.coverAlt', { name: continueBook.name })"
            />
          </RouterLink>
          <div class="continue-copy">
            <div class="continue-heading">
              <div>
                <h3 :id="`continue-${continueBook.id}`">
                  {{ continueBook.name }}
                </h3>
                <p>
                  {{ continueBook.author || $t("app.common.unknownAuthor") }}
                </p>
              </div>
              <span>{{
                $t("shelf.chapterOf", {
                  chapter: chapter(continueBook),
                  total: continueBook.totalChapterNum || "—",
                })
              }}</span>
            </div>
          </div>
          <div class="continue-reading">
            <p class="current-chapter">
              <span>{{ $t("shelf.current") }}</span><strong>{{ currentChapter(continueBook) }}</strong>
            </p>
            <div
              class="progress-track"
              role="progressbar"
              :aria-label="$t('shelf.progressFor', { name: continueBook.name })"
              :aria-valuenow="progress(continueBook)"
              aria-valuemin="0"
              aria-valuemax="100"
            >
              <span
                :style="{
                  transform: `scaleX(${progress(continueBook) / 100})`,
                }"
              />
            </div>
            <div class="app-actions continue-actions">
              <RouterLink
                class="app-button app-button--primary"
                :to="readerResumeLocation(continueBook)"
              >
                <AppIcon name="book" />{{ $t("shelf.continue") }}
</RouterLink><RouterLink
                class="app-button app-button--secondary"
                :to="`/books/${encodeURIComponent(continueBook.id)}`"
              >
                {{ $t("shelf.details") }}
              </RouterLink>
            </div>
          </div>
        </div>
      </section>

      <section class="shelf-section" :aria-label="$t('shelf.booksLabel')">
        <header>
          <div>
            <h2>{{ $t("shelf.booksLabel") }}</h2>
            <p>{{ $t("shelf.collectionDescription") }}</p>
          </div>
          <span>{{ $t("shelf.bookCount", { count: books.length }) }}</span>
        </header>
        <div class="shelf-tools">
          <label><span>{{ $t('shelf.filterLabel') }}</span><input v-model="query" type="search" :placeholder="$t('shelf.filterPlaceholder')"></label>
          <label><span>{{ $t('shelf.sortLabel') }}</span><select v-model="sort"><component :is="'button'" type="button"><selectedcontent /></component><option value="recent">{{ $t('shelf.sortRecent') }}</option><option value="title">{{ $t('shelf.sortTitle') }}</option><option value="author">{{ $t('shelf.sortAuthor') }}</option><option value="progress">{{ $t('shelf.sortProgress') }}</option></select></label>
        </div>
        <section v-if="!visibleBooks.length" class="no-matches"><p>{{ $t('shelf.noMatches') }}</p><AppButton variant="secondary" @click="clearQuery">{{ $t('shelf.clearFilter') }}</AppButton></section>
        <div v-else class="book-grid">
          <article v-for="book in visibleBooks" :key="book.id" class="book-card">
            <RouterLink
              class="shelf-cover"
              :to="`/books/${encodeURIComponent(book.id)}`"
              :aria-label="$t('shelf.detailsFor', { name: book.name })"
            >
              <BookCover :name="book.name" :url="coverURL(book)" alt="" lazy />
            </RouterLink>
            <div class="book-copy">
              <RouterLink :to="`/books/${encodeURIComponent(book.id)}`">
                <strong>{{ book.name }}</strong>
</RouterLink><span>{{ book.author || $t("app.common.unknownAuthor") }}</span>
              <p>
                <small>{{ $t("shelf.current") }}</small><b>{{ currentChapter(book) }}</b>
              </p>
              <p v-if="latestChapter(book)" class="latest">
                <small>{{ $t("shelf.latestLabel") }}</small><span>{{ latestChapter(book) }}</span>
              </p>
            </div>
            <div class="book-footer">
              <div>
                <span>{{
                  $t("shelf.chapterOf", {
                    chapter: chapter(book),
                    total: book.totalChapterNum || "—",
                  })
                }}</span><strong>{{ progress(book) }}%</strong>
              </div>
              <div class="progress-track" aria-hidden="true">
                <span
                  :style="{ transform: `scaleX(${progress(book) / 100})` }"
                />
              </div>
              <RouterLink
                class="resume app-button app-button--secondary"
                :to="readerResumeLocation(book)"
              >
                <AppIcon name="book" />{{ $t("shelf.resume") }}
              </RouterLink>
            </div>
          </article>
        </div>
      </section>
    </div>
  </FeatureScaffold>
</template>

<style scoped>
.library {
  display: grid;
  gap: clamp(2.25rem, 5vw, 4rem);
}
.state {
  padding: 2rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-paper-raised);
  text-align: center;
}
.state div {
  display: flex;
  justify-content: center;
  gap: 1rem;
}
.continue-section > header,
.shelf-section > header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.85rem;
}
.continue-section > header h2,
.shelf-section h2 {
  margin: 0;
  font: var(--weight-strong) var(--text-section) var(--font-literary);
}
.continue-section > header > span,
.shelf-section > header > span {
  color: var(--color-ink-muted);
  font-size: var(--text-small);
  font-variant-numeric: tabular-nums;
}
.shelf-section > header p {
  margin: 0.2rem 0 0;
  color: var(--color-ink-muted);
  font-size: var(--text-small);
}
.continue-panel {
  display: grid;
  grid-template-columns: 11rem minmax(0, 1fr);
  grid-template-areas: "cover identity" "cover reading";
  column-gap: clamp(1.25rem, 3vw, 2rem);
  align-items: stretch;
  padding: clamp(1.1rem, 2.5vw, 1.6rem);
  border: 1px solid
    color-mix(in srgb, var(--color-warm) 34%, var(--color-border));
  border-radius: var(--radius-lg);
  background: color-mix(
    in srgb,
    var(--color-paper-raised) 82%,
    var(--color-accent-soft)
  );
  box-shadow: var(--shadow-card);
}
.continue-cover,
.shelf-cover {
  display: block;
  overflow: hidden;
  background: var(--color-accent);
  text-decoration: none;
}
.continue-cover {
  grid-area: cover;
  width: 11rem;
  height: auto;
  aspect-ratio: 3/4;
  align-self: center;
  border-radius: 0;
  box-shadow: 0 0.55rem 1.1rem rgb(54 39 26/0.17);
}
.continue-cover :deep(.book-cover),
.shelf-cover :deep(.book-cover) {
  width: 100%;
  height: 100%;
}
.continue-copy {
  grid-area: identity;
  min-width: 0;
  align-self: end;
}
.continue-reading {
  grid-area: reading;
  min-width: 0;
}
.continue-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 1rem;
}
.continue-heading h3 {
  margin: 0;
  font: var(--weight-strong) var(--text-subheading)/1.18 var(--font-literary);
  overflow-wrap: anywhere;
}
.continue-heading p {
  margin: 0.35rem 0 0;
  color: var(--color-ink-muted);
}
.continue-heading > span {
  flex: none;
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: var(--color-paper-muted);
  color: var(--color-ink-muted);
  font-size: var(--text-caption);
  font-variant-numeric: tabular-nums;
}
.current-chapter {
  display: grid;
  gap: 0.15rem;
  margin: clamp(1rem, 2.5vw, 1.7rem) 0 0.75rem;
}
.current-chapter span,
.book-copy small {
  color: var(--color-warm);
  font-size: var(--text-caption);
  font-weight: var(--weight-strong);
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.current-chapter strong {
  font: var(--weight-strong) var(--text-body)/1.4 var(--font-literary);
  overflow-wrap: anywhere;
}
.progress-track {
  height: 0.35rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--color-paper-muted);
}
.progress-track span {
  display: block;
  width: 100%;
  height: 100%;
  transform-origin: left;
  background: var(--color-warm);
}
.continue-actions {
  margin-top: 1rem;
}
.shelf-tools { display: grid; grid-template-columns: minmax(0, 1fr) minmax(11rem, 18rem); gap: 1rem; align-items: end; margin-bottom: 1rem; padding: 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.shelf-tools label { min-width: 0; display: grid; gap: .3rem; }
.shelf-tools label > span { color: var(--color-ink-muted); font-size: var(--text-caption); font-weight: var(--weight-strong); }
.shelf-tools input, .shelf-tools select { width: 100%; min-width: 0; max-width: 100%; min-height: 2.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .55rem .7rem; background: white; color: var(--color-ink); font: var(--weight-regular) var(--text-body)/1.25 var(--font-ui); }
.shelf-tools select { --select-radius: var(--radius-md); align-items: center; }
.no-matches { display: grid; justify-items: center; gap: .75rem; padding: 2rem 1rem; border: 1px dashed var(--color-border); color: var(--color-ink-muted); text-align: center; }
.no-matches p { margin: 0; }
.book-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(13.5rem, 1fr));
  gap: clamp(1.25rem, 2.5vw, 2rem);
}
.book-card {
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding: 0.55rem 0.55rem 0.7rem;
  border: 1px solid color-mix(in srgb, var(--color-border) 72%, transparent);
  background: color-mix(in srgb, var(--color-paper-raised) 68%, transparent);
  box-shadow: 0 0.45rem 1rem rgb(54 39 26 / 0.07);
  transition: border-color 0.2s ease-out, box-shadow 0.2s ease-out, transform 0.2s ease-out;
}
.book-card:hover,
.book-card:focus-within {
  border-color: color-mix(in srgb, var(--color-warm) 38%, var(--color-border));
  box-shadow: 0 0.75rem 1.5rem rgb(54 39 26 / 0.12);
  transform: translateY(-0.15rem);
}
.shelf-cover {
  width: 100%;
  aspect-ratio: 3/4;
  border-radius: 0;
  box-shadow: inset 0.22rem 0 rgb(255 255 255 / 0.12), inset -0.08rem 0 rgb(0 0 0 / 0.12), 0 0.5rem 1rem rgb(54 39 26/0.14);
  transition:
    transform 0.2s ease-out,
    box-shadow 0.2s ease-out;
}
.book-card:hover .shelf-cover,
.book-card:focus-within .shelf-cover {
  transform: translateY(-0.12rem);
  box-shadow: inset 0.22rem 0 rgb(255 255 255 / 0.14), inset -0.08rem 0 rgb(0 0 0 / 0.14), 0 0.75rem 1.35rem rgb(54 39 26/0.18);
}
.book-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding: 0.85rem 0.1rem 0.7rem;
}
.book-copy > a {
  color: var(--color-ink);
  text-decoration: none;
}
.book-copy strong {
  display: -webkit-box;
  overflow: hidden;
  font: var(--weight-strong) var(--text-subheading)/1.35 var(--font-literary);
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}
.book-copy > span {
  margin-top: 0.2rem;
  color: var(--color-ink-muted);
  font-size: var(--text-small);
}
.book-copy p {
  display: grid;
  gap: 0.12rem;
  margin: 0.7rem 0 0;
  min-width: 0;
}
.book-copy p b,
.book-copy p > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--text-small);
}
.book-copy p b {
  font-weight: var(--weight-strong);
}
.book-copy .latest {
  margin-top: 0.5rem;
  color: var(--color-ink-muted);
}
.book-footer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.55rem 0.75rem;
  margin-top: auto;
}
.book-footer > div:first-child {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  color: var(--color-ink-muted);
  font-size: var(--text-caption);
  font-variant-numeric: tabular-nums;
}
.book-footer > div:first-child strong {
  color: var(--color-ink);
}
.book-footer > .progress-track {
  grid-column: 1;
}
.resume {
  grid-column: 2;
  grid-row: 1/3;
  align-self: end;
  padding-inline: .75rem;
}
.book-copy > a:hover {
  text-decoration: underline;
  text-underline-offset: 0.2em;
}
@media (max-width: 48rem) {
  .continue-panel {
    grid-template-columns: 8rem minmax(0, 1fr);
  }
  .continue-cover {
    width: 8rem;
  }
  .book-grid {
    grid-template-columns: repeat(auto-fill, minmax(10.5rem, 1fr));
    gap: 1.25rem;
  }
}
@media (max-width: 34rem) {
  .shelf-tools { grid-template-columns: 1fr; }
  .continue-panel {
    grid-template-columns: clamp(4.5rem, 20vw, 6rem) minmax(0, 1fr);
    grid-template-areas: "cover identity" "reading reading";
    gap: 1rem;
  }
  .continue-cover {
    width: 100%;
    align-self: start;
  }
  .continue-copy {
    align-self: start;
  }
  .current-chapter {
    margin-top: 0;
  }
  .continue-heading {
    display: block;
  }
  .continue-heading > span {
    display: inline-flex;
    margin-top: 0.65rem;
  }
  .continue-actions .app-button--primary {
    flex: 1 1 10rem;
  }
  .book-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 1rem;
  }
  .book-footer {
    grid-template-columns: 1fr;
  }
  .resume {
    grid-column: 1;
    grid-row: auto;
    width: 100%;
  }
  .book-footer > .progress-track {
    grid-column: 1;
  }
  .shelf-section > header {
    align-items: start;
  }
  .state div {
    flex-direction: column;
  }
}
@media (max-width: 22rem) {
  .book-grid {
    grid-template-columns: 1fr;
  }
}
</style>
