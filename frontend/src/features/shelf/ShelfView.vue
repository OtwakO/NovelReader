<script lang="ts">
import { defineComponent, nextTick } from "vue";
import { listBooks } from "../../api/books";
import type { Book } from "../../api/models";
import AppButton from "../../ui/components/AppButton.vue";
import FeatureScaffold from "../../ui/components/FeatureScaffold.vue";
import { readableChapterLabel } from "../books/book-display";
import { currentChapterNumber, shelfProgressPercent } from "./shelf-progress";
import { loadShelfViewState, saveShelfViewState, visibleShelfBooks, type ShelfSort } from "./shelf-view-state";

export default defineComponent({
  name: "ShelfView",
  components: { AppButton, FeatureScaffold },
  data() {
    const view = loadShelfViewState();
    return { books: [] as Book[], loading: true, error: "", query: view.query, sort: view.sort as ShelfSort, restoreScrollY: view.scrollY };
  },
  computed: {
    continueBook(): Book | null {
      return visibleShelfBooks(this.books, '', 'recent')[0] ?? null;
    },
    visibleBooks(): Book[] { return visibleShelfBooks(this.books, this.query, this.sort); },
  },
  watch: {
    query() { this.saveView(); },
    sort() { this.saveView(); },
  },
  async mounted() {
    window.addEventListener('scroll', this.captureScroll, { passive: true });
    await this.load();
    await nextTick();
    window.scrollTo({ top: this.restoreScrollY });
  },
  beforeUnmount() { this.captureScroll(); window.removeEventListener('scroll', this.captureScroll); },
  methods: {
    saveView(scrollY = window.scrollY) { saveShelfViewState({ query: this.query, sort: this.sort, scrollY }); },
    captureScroll() { this.saveView(); },
    clearQuery() { this.query = ''; },
    async load() {
      this.loading = true;
      this.error = "";
      try {
        this.books = await listBooks();
      } catch (cause) {
        this.error =
          cause instanceof Error ? cause.message : this.$t("shelf.failed");
      } finally {
        this.loading = false;
      }
    },
    progress(book: Book) {
      return shelfProgressPercent(book);
    },
    chapter(book: Book) {
      return currentChapterNumber(book);
    },
    latestChapter(book: Book) { return readableChapterLabel(book.lastChapter); },
    currentChapter(book: Book) {
      return (
        book.currentChapterTitle ||
        this.$t("shelf.chapter", { chapter: this.chapter(book) })
      );
    },
    coverURL(book: Book) { return book.coverDisplayUrl || ''; },
    coverFailed(event: Event) {
      (event.currentTarget as HTMLImageElement).classList.add("image-failed");
    },
  },
});
</script>

<template>
  <FeatureScaffold
    :title="$t('shelf.title')"
    :description="$t('shelf.description')"
  >
    <p v-if="loading" aria-busy="true">{{ $t("shelf.loading") }}</p>
    <section v-else-if="error" class="state">
      <p role="alert">{{ error }}</p>
      <AppButton variant="secondary" @click="load">
        {{ $t("app.common.retry") }}
      </AppButton>
    </section>
    <section v-else-if="books.length === 0" class="state">
      <h2>{{ $t("shelf.emptyTitle") }}</h2>
      <p>{{ $t("shelf.emptyDescription") }}</p>
      <div>
        <RouterLink to="/explore">{{ $t("shelf.explore") }}</RouterLink><RouterLink to="/search">{{ $t("shelf.search") }}</RouterLink>
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
            <span aria-hidden="true">{{ continueBook.name.slice(0, 1) }}</span>
            <img
              v-if="coverURL(continueBook)"
              :src="coverURL(continueBook)"
              :alt="$t('shelf.coverAlt', { name: continueBook.name })"
              @error="coverFailed"
            >
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
            <div class="continue-actions">
              <RouterLink
                class="continue-action"
                :to="{
                  name: 'reader',
                  params: {
                    bookId: continueBook.id,
                    chapterIndex: Math.max(0, continueBook.durChapterIndex),
                  },
                }"
              >
                {{ $t("shelf.continue")
                }}<span aria-hidden="true">→</span>
</RouterLink><RouterLink
                class="detail-action"
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
          <label><span>{{ $t('shelf.sortLabel') }}</span><select v-model="sort"><option value="recent">{{ $t('shelf.sortRecent') }}</option><option value="title">{{ $t('shelf.sortTitle') }}</option><option value="author">{{ $t('shelf.sortAuthor') }}</option><option value="progress">{{ $t('shelf.sortProgress') }}</option></select></label>
        </div>
        <section v-if="!visibleBooks.length" class="no-matches"><p>{{ $t('shelf.noMatches') }}</p><AppButton variant="secondary" @click="clearQuery">{{ $t('shelf.clearFilter') }}</AppButton></section>
        <div v-else class="book-grid">
          <article v-for="book in visibleBooks" :key="book.id" class="book-card">
            <RouterLink
              class="book-cover"
              :to="`/books/${encodeURIComponent(book.id)}`"
              :aria-label="$t('shelf.detailsFor', { name: book.name })"
            >
              <span aria-hidden="true">{{ book.name.slice(0, 1) }}</span>
              <img
                v-if="coverURL(book)"
                :src="coverURL(book)"
                alt=""
                loading="lazy"
                @error="coverFailed"
              >
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
                class="resume"
                :to="{
                  name: 'reader',
                  params: {
                    bookId: book.id,
                    chapterIndex: Math.max(0, book.durChapterIndex),
                  },
                }"
              >
                {{ $t("shelf.resume") }}
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
  font: 700 clamp(1.35rem, 3vw, 1.8rem) var(--font-literary);
}
.continue-section > header > span,
.shelf-section > header > span {
  color: var(--color-ink-muted);
  font-size: 0.82rem;
  font-variant-numeric: tabular-nums;
}
.shelf-section > header p {
  margin: 0.2rem 0 0;
  color: var(--color-ink-muted);
  font-size: 0.85rem;
}
.continue-panel {
  display: grid;
  grid-template-columns: 8rem minmax(0, 1fr);
  gap: clamp(1.25rem, 3vw, 2rem);
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
.book-cover {
  position: relative;
  display: grid;
  place-items: center;
  overflow: hidden;
  background: var(--color-accent);
  color: white;
  font: 700 1.7rem var(--font-literary);
  text-decoration: none;
}
.continue-cover {
  width: 8rem;
  aspect-ratio: 2/3;
  border-radius: 0;
  box-shadow: 0 0.55rem 1.1rem rgb(54 39 26/0.17);
}
.continue-cover img,
.book-cover img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}
.continue-cover img {
  object-fit: contain;
  background: var(--color-paper-muted);
}
.book-cover img {
  object-fit: cover;
}
.continue-cover img.image-failed,
.book-cover img.image-failed {
  display: none;
}
.continue-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
}
.continue-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 1rem;
}
.continue-heading h3 {
  margin: 0;
  font: 700 clamp(1.55rem, 3.4vw, 2.25rem)/1.18 var(--font-literary);
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
  background: var(--color-paper-raised);
  color: var(--color-ink-muted);
  font-size: 0.76rem;
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
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.current-chapter strong {
  font: 700 clamp(1rem, 2vw, 1.18rem)/1.4 var(--font-literary);
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
  display: flex;
  gap: 0.55rem;
  margin-top: 1rem;
}
.continue-action,
.detail-action,
.resume {
  min-height: 2.75rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  font-weight: 800;
  text-decoration: none;
}
.continue-action {
  min-width: 11rem;
  gap: 0.55rem;
  padding: 0.7rem 1.2rem;
  background: var(--color-accent);
  color: white;
}
.continue-action:hover {
  background: var(--color-accent-strong);
}
.detail-action {
  padding: 0.7rem 1rem;
  border: 1px solid color-mix(in srgb, var(--color-accent) 56%, var(--color-border));
  background: var(--color-paper-raised);
  color: var(--color-accent-strong);
  box-shadow: 0 0.2rem 0.5rem rgb(54 39 26 / 0.08);
  transition: background 0.18s ease-out, border-color 0.18s ease-out, box-shadow 0.18s ease-out;
}
.detail-action:hover,
.detail-action:focus-visible {
  border-color: var(--color-accent);
  background: color-mix(in srgb, var(--color-accent-soft) 55%, var(--color-paper-raised));
  box-shadow: 0 0.35rem 0.75rem rgb(54 39 26 / 0.12);
}
.shelf-tools { display: grid; grid-template-columns: minmax(0, 1fr) minmax(11rem, auto); gap: .75rem; align-items: end; margin-bottom: 1rem; padding: .85rem; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
.shelf-tools label { display: grid; gap: .3rem; color: var(--color-ink-muted); font-size: .75rem; font-weight: 700; }
.shelf-tools input, .shelf-tools select { width: 100%; min-height: 2.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: .55rem .7rem; background: white; color: var(--color-ink); font: inherit; }
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
.book-cover {
  width: 100%;
  aspect-ratio: 2/3;
  border-radius: 0;
  box-shadow: inset 0.22rem 0 rgb(255 255 255 / 0.12), inset -0.08rem 0 rgb(0 0 0 / 0.12), 0 0.5rem 1rem rgb(54 39 26/0.14);
  transition:
    transform 0.2s ease-out,
    box-shadow 0.2s ease-out;
}
.book-card:hover .book-cover,
.book-card:focus-within .book-cover {
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
  font: 700 1.08rem/1.35 var(--font-literary);
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}
.book-copy > span {
  margin-top: 0.2rem;
  color: var(--color-ink-muted);
  font-size: 0.82rem;
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
  font-size: 0.82rem;
}
.book-copy p b {
  font-weight: 700;
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
  font-size: 0.75rem;
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
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border);
  color: var(--color-accent);
}
.book-copy > a:hover,
.resume:hover {
  text-decoration: underline;
  text-underline-offset: 0.2em;
}
@media (max-width: 48rem) {
  .continue-panel {
    grid-template-columns: 6.5rem minmax(0, 1fr);
  }
  .continue-cover {
    width: 6.5rem;
  }
  .book-grid {
    grid-template-columns: repeat(auto-fill, minmax(10.5rem, 1fr));
    gap: 1.25rem;
  }
}
@media (max-width: 34rem) {
  .shelf-tools { grid-template-columns: 1fr; }
  .continue-panel {
    grid-template-columns: 5.25rem minmax(0, 1fr);
    gap: 1rem;
  }
  .continue-cover {
    width: 5.25rem;
  }
  .continue-heading {
    display: block;
  }
  .continue-heading > span {
    display: inline-flex;
    margin-top: 0.65rem;
  }
  .continue-actions {
    grid-column: 1/-1;
    flex-direction: column;
  }
  .continue-action,
  .detail-action {
    width: 100%;
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
